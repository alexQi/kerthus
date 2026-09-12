package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	me "go-micro.dev/v6/errors"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/gateway"
	"kerthus/internal/platform/rpcauth"
	file "kerthus/internal/saas/domain/file"
)

const defaultAttachmentMaxBytes int64 = 50 << 20

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Open(context.Context, string) (io.ReadCloser, int64, string, time.Time, error)
}

type Config struct {
	AttachmentMaxBytes   int64
	MaxConcurrentUploads int
	// TempDir can isolate staging files; files always have owner-only permissions.
	TempDir string
}

func Handler(client pb.PlatformService, store ObjectStore, key string) http.Handler {
	return HandlerWithConfig(client, store, key, Config{})
}

func HandlerWithConfig(client pb.PlatformService, store ObjectStore, key string, cfg Config) http.Handler {
	if cfg.AttachmentMaxBytes <= 0 {
		cfg.AttachmentMaxBytes = defaultAttachmentMaxBytes
	}
	if cfg.MaxConcurrentUploads <= 0 {
		cfg.MaxConcurrentUploads = 4
	}
	slots := make(chan struct{}, cfg.MaxConcurrentUploads)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/file/limits" && r.Method == http.MethodGet {
			answer(w, map[string]any{"image_max_bytes": maxSize, "attachment_max_bytes": cfg.AttachmentMaxBytes}, nil)
			return
		}
		if r.URL.Path != "/app/file/upload" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			w.Header().Set("Retry-After", "2")
			answer(w, nil, me.New("upload", "上传繁忙，请稍后重试", 429))
			return
		}
		actor, err := gateway.RequestContext(r)
		if err != nil {
			gateway.WriteError(w, err)
			return
		}
		ctx := rpcauth.WithCredential(r.Context(), key)
		// Validate the session before accepting a potentially large request body.
		if _, err = client.Auth(ctx, actor); err != nil {
			answer(w, nil, err)
			return
		}
		limit := cfg.AttachmentMaxBytes
		if limit < maxSize {
			limit = maxSize
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit+(64<<10))
		reader, err := r.MultipartReader()
		if err != nil {
			answer(w, nil, me.BadRequest("upload", "文件请求格式无效"))
			return
		}
		var staged *os.File
		defer func() {
			if staged != nil {
				staged.Close()
				os.Remove(staged.Name())
			}
		}()
		var filename, purpose string
		var size int64
		metadataBytes := 0
		for parts := 0; ; parts++ {
			part, e := reader.NextPart()
			if e == io.EOF {
				break
			}
			if e != nil || parts >= 32 {
				answer(w, nil, me.BadRequest("upload", "文件请求格式无效或超限"))
				return
			}
			if part.FormName() == "file" && part.FileName() != "" {
				if staged != nil {
					part.Close()
					answer(w, nil, me.BadRequest("upload", "只能上传一个文件"))
					return
				}
				filename = file.SafeFilename(part.FileName())
				staged, err = os.CreateTemp(cfg.TempDir, "kerthus-upload-*")
				if err != nil {
					part.Close()
					answer(w, nil, err)
					return
				}
				size, err = io.CopyBuffer(staged, io.LimitReader(part, limit+1), make([]byte, 64<<10))
				// Do not drain an oversized stream: the bounded request is closed by the server.
				if err != nil || size <= 0 || size > limit {
					answer(w, nil, me.BadRequest("upload", "文件为空或超出大小限制"))
					return
				}
			} else {
				value, e := io.ReadAll(io.LimitReader(part, 4097))
				metadataBytes += len(value)
				if e != nil || len(value) > 4096 || metadataBytes > 32768 {
					answer(w, nil, me.BadRequest("upload", "表单字段超限"))
					return
				}
				if part.FormName() == "purpose" {
					purpose = string(value)
				}
				// Legacy directory/param are deliberately accepted but never used as paths.
			}
			part.Close()
		}
		if staged == nil {
			answer(w, nil, me.BadRequest("upload", "缺少file字段"))
			return
		}
		if purpose != "" && purpose != "image" && purpose != "attachment" {
			answer(w, nil, me.BadRequest("upload", "purpose 必须为 image 或 attachment"))
			return
		}
		if _, err = staged.Seek(0, io.SeekStart); err != nil {
			answer(w, nil, err)
			return
		}
		probe := make([]byte, 512)
		n, err := staged.Read(probe)
		if err != nil && err != io.EOF {
			answer(w, nil, err)
			return
		}
		ct := http.DetectContentType(probe[:n])
		isImage := ct == "image/png" || ct == "image/jpeg" || ct == "image/webp"
		private := purpose == "attachment" || !isImage
		if purpose == "image" && !isImage {
			answer(w, nil, me.BadRequest("upload", "仅支持PNG、JPEG、WebP图片"))
			return
		}
		allowed := maxSize
		if private {
			allowed = cfg.AttachmentMaxBytes
		}
		if size > allowed {
			answer(w, nil, me.BadRequest("upload", fmt.Sprintf("文件大小不能超过 %d 字节", allowed)))
			return
		}
		pending, err := client.CreateUpload(ctx, &pb.UploadRequest{Context: actor, ContentType: ct, Size: size, Filename: filename, Private: private})
		if err != nil {
			answer(w, nil, err)
			return
		}
		confirmed := false
		defer func() {
			if !confirmed {
				cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				// Keys are generated by the core, including the resolved tenant/app
				// even when the browser selected its defaults with zero context IDs.
				segments := strings.Split(pending.ObjectKey, "/")
				if len(segments) < 4 {
					log.Print("upload cleanup: invalid server object key")
					return
				}
				tenantID, _ := strconv.ParseInt(segments[1], 10, 64)
				appID, _ := strconv.ParseInt(segments[2], 10, 64)
				_, cleanupErr := client.CancelUpload(rpcauth.WithCredential(cleanup, key), &pb.FileRequest{Context: &pb.Context{TenantId: tenantID, AppId: appID}, Id: pending.Id, ObjectKey: pending.ObjectKey})
				if cleanupErr != nil {
					log.Print("upload cleanup incomplete; pending file requires retry")
				}
			}
		}()
		if _, err = staged.Seek(0, io.SeekStart); err != nil {
			answer(w, nil, err)
			return
		}
		if err = store.Put(ctx, pending.ObjectKey, staged, size, ct); err != nil {
			answer(w, nil, err)
			return
		}
		result, err := client.ConfirmUpload(ctx, &pb.ConfirmUploadRequest{Context: actor, Id: pending.Id})
		if err != nil {
			answer(w, nil, err)
			return
		}
		confirmed = true
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": result.ObjectKey, "msg": "", "mesc": "", "file": map[string]any{
			"id": result.Id, "filename": result.Filename, "size": result.Size, "content_type": result.ContentType, "private": result.Private, "max_size": result.MaxSize,
			"download_path": "/app/file/download?key=" + url.QueryEscape(result.ObjectKey),
		}})
	})
}

// Download never uses a bearer token in the URL. Browser clients fetch with the
// existing context headers and turn the response into a local Blob URL.
func Download(client pb.PlatformService, store ObjectStore, key string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		actor, err := gateway.RequestContext(r)
		if err != nil {
			gateway.WriteError(w, err)
			return
		}
		var id int64
		if raw := r.URL.Query().Get("id"); raw != "" {
			id, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				answer(w, nil, me.BadRequest("download", "文件标识无效"))
				return
			}
		}
		ctx := rpcauth.WithCredential(r.Context(), key)
		result, err := client.ResolveDownload(ctx, &pb.FileRequest{Context: actor, Id: id, ObjectKey: r.URL.Query().Get("key")})
		if err != nil {
			answer(w, nil, err)
			return
		}
		body, size, ct, _, err := store.Open(ctx, result.ObjectKey)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer body.Close()
		if size != result.Size || ct != result.ContentType {
			answer(w, nil, me.New("download", "文件元数据不一致", 503))
			return
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.SafeFilename(result.Filename)}))
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == http.MethodGet {
			_, _ = io.Copy(w, body)
		}
	})
}
