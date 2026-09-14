// Package routing forwards only registered, authorized application operations.
package routing

import (
	"encoding/json"
	"fmt"
	me "go-micro.dev/v6/errors"
	"go-micro.dev/v6/registry"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/platform/rpcauth"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Proxy struct {
	Platform   pb.PlatformService
	Registry   registry.Registry
	GatewayKey string
	Transport  http.RoundTripper
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fail := func(code int32, msg string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(int(code))
		json.NewEncoder(w).Encode(map[string]any{"code": code, "data": nil, "msg": msg, "mesc": msg})
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 6 || parts[1] != "api" || parts[2] != "apps" || parts[3] == "" || parts[4] != "v1" || strings.Contains(r.URL.EscapedPath(), "%") || strings.Contains(r.URL.Path, "//") {
		fail(404, "未注册的应用路径")
		return
	}
	id := func(k string) int64 { n, _ := strconv.ParseInt(r.Header.Get(k), 10, 64); return n }
	ctx := rpcauth.WithCredential(r.Context(), p.GatewayKey)
	access, e := p.Platform.CheckAccess(ctx, &pb.AccessRequest{Context: &pb.Context{Token: r.Header.Get("access-token"), TenantId: id("tenant-id"), AppId: id("app-id"), UnitId: id("unit-id"), SectionId: id("section-id"), RequestId: r.Header.Get("X-Request-Id")}, AppCode: parts[3], Method: r.Method, Path: r.URL.Path})
	if e != nil {
		err := me.FromError(e)
		code := err.Code
		if code < 400 || code > 599 {
			code = 503
		}
		msg := err.Detail
		if code >= 500 {
			msg = "平台授权服务暂不可用"
		}
		fail(code, msg)
		return
	}
	if access.RoutePrefix != "/api/apps/"+parts[3]+"/v1" || access.ServiceKey == "" {
		fail(503, "应用路由尚未配置")
		return
	}
	services, e := p.Registry.GetService(access.ServiceKey)
	if e != nil {
		fail(503, "应用服务暂不可用")
		return
	}
	var upstream *url.URL
	for _, svc := range services {
		for _, node := range svc.Nodes {
			if node.Metadata["protocol"] != "http" || node.Metadata["app_code"] != parts[3] {
				continue
			}
			if _, _, e = net.SplitHostPort(node.Address); e != nil {
				continue
			}
			upstream = &url.URL{Scheme: "http", Host: node.Address}
			break
		}
		if upstream != nil {
			break
		}
	}
	if upstream == nil {
		fail(503, "应用服务尚未就绪")
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	base := proxy.Director
	proxy.Director = func(req *http.Request) {
		base(req)
		for k := range req.Header {
			lower := strings.ToLower(k)
			if strings.HasPrefix(lower, "x-kerthus-") || strings.HasPrefix(lower, "x-forwarded-") || lower == "forwarded" || lower == "access-token" || lower == "tenant-id" || lower == "app-id" || lower == "unit-id" || lower == "section-id" || lower == "x-request-id" {
				req.Header.Del(k)
			}
		}
		req.Header.Set("tenant-id", fmt.Sprint(access.TenantId))
		req.Header.Set("app-id", fmt.Sprint(access.AppId))
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) { fail(503, "应用服务暂不可用") }
	proxy.Transport = p.Transport
	if proxy.Transport == nil {
		proxy.Transport = &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext, ResponseHeaderTimeout: 10 * time.Second, MaxIdleConns: 64, IdleConnTimeout: 60 * time.Second}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	proxy.ServeHTTP(w, r)
}
