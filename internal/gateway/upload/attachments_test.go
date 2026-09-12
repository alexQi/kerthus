package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go-micro.dev/v6/client"
	me "go-micro.dev/v6/errors"
	pb "kerthus/gen/go/saas/v1"
)

type filePlatform struct {
	pb.PlatformService
	upload                           *pb.UploadRequest
	reply                            *pb.UploadReply
	authErr, confirmErr, downloadErr error
	canceled                         int
}

func (m *filePlatform) Auth(context.Context, *pb.Context, ...client.CallOption) (*pb.AuthReply, error) {
	return &pb.AuthReply{}, m.authErr
}
func (m *filePlatform) CreateUpload(_ context.Context, r *pb.UploadRequest, _ ...client.CallOption) (*pb.UploadReply, error) {
	m.upload = r
	key := "public/2/3/" + strings.Repeat("a", 64) + ".png"
	if r.Private {
		key = "private/2/3/" + strings.Repeat("a", 64)
	}
	m.reply = &pb.UploadReply{Id: 12, ObjectKey: key, Filename: r.Filename, Size: r.Size, ContentType: r.ContentType, Private: r.Private, MaxSize: 50 << 20}
	return m.reply, nil
}
func (m *filePlatform) ConfirmUpload(context.Context, *pb.ConfirmUploadRequest, ...client.CallOption) (*pb.UploadReply, error) {
	return m.reply, m.confirmErr
}
func (m *filePlatform) CancelUpload(context.Context, *pb.FileRequest, ...client.CallOption) (*pb.Result, error) {
	m.canceled++
	return &pb.Result{Success: true}, nil
}
func (m *filePlatform) ResolveDownload(context.Context, *pb.FileRequest, ...client.CallOption) (*pb.UploadReply, error) {
	return m.reply, m.downloadErr
}

type memoryObjects struct {
	body             []byte
	ct, key          string
	putErr           error
	opened           int
	entered, release chan struct{}
}

func (s *memoryObjects) Put(_ context.Context, key string, r io.Reader, _ int64, ct string) error {
	if s.entered != nil {
		close(s.entered)
		<-s.release
	}
	s.key = key
	s.ct = ct
	s.body, _ = io.ReadAll(r)
	return s.putErr
}
func (s *memoryObjects) Open(context.Context, string) (io.ReadCloser, int64, string, time.Time, error) {
	s.opened++
	return io.NopCloser(bytes.NewReader(s.body)), int64(len(s.body)), s.ct, time.Time{}, nil
}
func multipartRequest(t *testing.T, filename string, body []byte, fields map[string]string) *http.Request {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	part, e := mw.CreateFormFile("file", filename)
	if e != nil {
		t.Fatal(e)
	}
	part.Write(body)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	mw.Close()
	r := httptest.NewRequest("POST", "/app/file/upload", &b)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("access-token", "opaque")
	r.Header.Set("tenant-id", "2")
	r.Header.Set("app-id", "3")
	return r
}
func TestAttachmentUploadPreservesLegacyWireAndIgnoresPaths(t *testing.T) {
	for _, test := range []struct {
		name    string
		body    []byte
		purpose string
		private bool
	}{
		{"报告.pdf", []byte("%PDF-1.4\nfixture"), "", true},
		{"图像.png", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 20)...), "", false},
		{"私密.png", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 20)...), "attachment", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			platform := &filePlatform{}
			store := &memoryObjects{}
			tmp := t.TempDir()
			h := HandlerWithConfig(platform, store, "key", Config{TempDir: tmp})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, multipartRequest(t, test.name, test.body, map[string]string{"directory": "../../private/other", "param": "../../escape", "purpose": test.purpose}))
			if w.Code != 200 || platform.upload == nil || platform.upload.Private != test.private || !bytes.Equal(store.body, test.body) {
				t.Fatalf("upload failed: %d %s", w.Code, w.Body)
			}
			var out struct {
				Data string
				File struct {
					Filename, DownloadPath string
					Private                bool
				}
			}
			if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
				t.Fatal(e)
			}
			if out.Data != platform.reply.ObjectKey || out.File.Filename != test.name || out.File.Private != test.private || platform.canceled != 0 {
				t.Fatalf("legacy contract/metadata lost: %s", w.Body)
			}
			entries, _ := os.ReadDir(tmp)
			if len(entries) != 0 {
				t.Fatal("staged file leaked")
			}
		})
	}
}
func TestUploadBoundedAndCleansFailedTransfers(t *testing.T) {
	for _, test := range []struct {
		name               string
		auth, put, confirm error
		body               []byte
		purpose            string
		want               int
		canceled           int
	}{
		{name: "unauthenticated", auth: me.Unauthorized("test", "no session"), body: []byte("data"), want: 401},
		{name: "attachment limit", body: bytes.Repeat([]byte("x"), 1025), purpose: "attachment", want: 400},
		{name: "image type", body: []byte("data"), purpose: "image", want: 400},
		{name: "unknown purpose", body: []byte("data"), purpose: "public", want: 400},
		{name: "put failure", body: []byte("data"), put: errors.New("storage failed"), want: 500, canceled: 1},
		{name: "confirm failure", body: []byte("data"), confirm: me.Forbidden("test", "revoked"), want: 403, canceled: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			platform := &filePlatform{authErr: test.auth, confirmErr: test.confirm}
			store := &memoryObjects{putErr: test.put}
			tmp := t.TempDir()
			h := HandlerWithConfig(platform, store, "key", Config{TempDir: tmp, AttachmentMaxBytes: 1024})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, multipartRequest(t, "fixture.txt", test.body, map[string]string{"purpose": test.purpose}))
			if w.Code != test.want || platform.canceled != test.canceled {
				t.Fatalf("status=%d cleanup=%d body=%s", w.Code, platform.canceled, w.Body)
			}
			entries, _ := os.ReadDir(tmp)
			if len(entries) != 0 {
				t.Fatal("staged file leaked")
			}
		})
	}
}
func TestConcurrentUploadLimit(t *testing.T) {
	platform := &filePlatform{}
	store := &memoryObjects{entered: make(chan struct{}), release: make(chan struct{})}
	h := HandlerWithConfig(platform, store, "key", Config{MaxConcurrentUploads: 1, TempDir: t.TempDir()})
	done := make(chan struct{})
	r := multipartRequest(t, "a.txt", []byte("test"), nil)
	go func() { defer close(done); h.ServeHTTP(httptest.NewRecorder(), r) }()
	<-store.entered
	w := httptest.NewRecorder()
	h.ServeHTTP(w, multipartRequest(t, "b.txt", []byte("test"), nil))
	close(store.release)
	<-done
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("upload concurrency was unbounded")
	}
}
func TestDownloadChineseAttachmentAndAuthorization(t *testing.T) {
	platform := &filePlatform{reply: &pb.UploadReply{ObjectKey: "private/2/3/key", Filename: "../财务报告\r\n.pdf", Size: 7, ContentType: "application/pdf", Private: true}}
	store := &memoryObjects{body: []byte("fixture"), ct: "application/pdf"}
	h := Download(platform, store, "key")
	for _, method := range []string{"GET", "HEAD"} {
		r := httptest.NewRequest(method, "/app/file/download?id=12", nil)
		r.Header.Set("access-token", "opaque")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		disposition, params, e := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
		if e != nil || disposition != "attachment" || params["filename"] != "财务报告.pdf" || !strings.Contains(w.Header().Get("Content-Disposition"), "filename*=utf-8''") {
			t.Fatalf("unsafe attachment filename: %q %v", w.Header().Get("Content-Disposition"), e)
		}
		if w.Code != 200 || w.Header().Get("Cache-Control") != "private, no-store" || (method == "HEAD" && w.Body.Len() != 0) || (method == "GET" && w.Body.String() != "fixture") {
			t.Fatalf("download=%d %s", w.Code, w.Body)
		}
	}
	platform.downloadErr = me.NotFound("test", "not found")
	opened := store.opened
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/app/file/download?key=other", nil))
	if w.Code != 404 || store.opened != opened {
		t.Fatal("unauthorized download reached object storage")
	}
}
func TestPrivateKeysNeverUsePublicOrLegacyDelivery(t *testing.T) {
	h := FilesWithLegacy(nil, "https://legacy.example/assets")
	for _, path := range []string{"/files/private/2/3/" + strings.Repeat("a", 64), "/files/private/tenant/document.pdf"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 || w.Header().Get("Location") != "" {
			t.Fatal("private object exposed through legacy delivery")
		}
	}
}

func TestPublicImageTenMiBLimitIsIndependentFromAttachments(t *testing.T) {
	body := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, (10<<20)-7)...)
	for _, purpose := range []string{"image", "attachment"} {
		platform := &filePlatform{}
		store := &memoryObjects{}
		h := HandlerWithConfig(platform, store, "key", Config{TempDir: t.TempDir()})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, multipartRequest(t, "large.png", body, map[string]string{"purpose": purpose}))
		if purpose == "image" && (w.Code != 400 || platform.upload != nil) {
			t.Fatal("oversized public image accepted")
		}
		if purpose == "attachment" && (w.Code != 200 || platform.upload == nil || !platform.upload.Private) {
			t.Fatalf("private image below attachment limit rejected: %d %s", w.Code, w.Body)
		}
	}
}
