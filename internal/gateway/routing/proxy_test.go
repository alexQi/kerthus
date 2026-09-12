package routing

import (
	"context"
	"go-micro.dev/v6/client"
	me "go-micro.dev/v6/errors"
	"go-micro.dev/v6/registry"

	"io"
	pb "kerthus/gen/go/saas/v1"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type platformStub struct {
	pb.PlatformService
	reply *pb.AccessReply
	err   error
	seen  *pb.AccessRequest
}

func (s *platformStub) CheckAccess(ctx context.Context, in *pb.AccessRequest, opts ...client.CallOption) (*pb.AccessReply, error) {
	s.seen = in
	return s.reply, s.err
}
func TestRegisteredAppProxyChecksBeforeForwarding(t *testing.T) {
	called := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("X-Kerthus-Actor") != "" || r.Header.Get("X-Kerthus-Service-Key") != "" {
			t.Error("forged service metadata forwarded")
		}
		if r.Header.Get("tenant-id") != "9" || r.Header.Get("app-id") != "7" {
			t.Error("context not canonicalized")
		}
		if r.URL.Path != "/api/apps/notes/v1/notes/123" {
			t.Error("path changed")
		}
		io.WriteString(w, "ok")
	}))
	defer backend.Close()
	reg := registry.NewMemoryRegistry()
	reg.Register(&registry.Service{Name: "kerthus.notes", Version: "1", Nodes: []*registry.Node{{Id: "notes-1", Address: strings.TrimPrefix(backend.URL, "http://"), Metadata: map[string]string{"protocol": "http", "app_code": "notes"}}}})
	stub := &platformStub{reply: &pb.AccessReply{TenantId: 9, AppId: 7, ServiceKey: "kerthus.notes", RoutePrefix: "/api/apps/notes/v1"}}
	proxy := &Proxy{Platform: stub, Registry: reg, GatewayKey: strings.Repeat("g", 32)}
	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/apps/notes/v1/notes/123", nil)
		r.Header.Set("X-Kerthus-Actor", "admin")
		r.Header.Set("X-Kerthus-Service-Key", "forged")
		r.Header.Set("tenant-id", "9")
		r.Header.Set("app-id", "7")
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)
		return w
	}
	w := request()
	if w.Code != 200 || w.Body.String() != "ok" || !called || stub.seen.AppCode != "notes" {
		t.Fatalf("forward failed %d %s", w.Code, w.Body.String())
	}
	called = false
	stub.err = me.Forbidden("access", "denied")
	w = request()
	if w.Code != 403 || called {
		t.Fatal("unauthorized request reached upstream")
	}
	stub.err = nil
	stub.reply.ServiceKey = "unavailable"
	w = request()
	if w.Code != 503 || called {
		t.Fatal("unavailable app must not change authorization or reach upstream")
	}
}
