// Package gateway adapts the existing Vue console's HTTP contracts to typed SaaS RPC.
package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	microerrors "go-micro.dev/v6/errors"
	"io"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/platform/rpcauth"
	"kerthus/internal/saas/domain/fault"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	GatewayKey, StaticURL             string
	CORSOrigins                       []string
	Upload, Download, Files, AppProxy http.Handler
}
type Gateway struct {
	client pb.PlatformService
	config Config
	routes map[string]route
}
type route struct {
	method string
	call   func(context.Context, *pb.Context, payload) (any, error)
	check  bool
}
type payload map[string]any

func New(client pb.PlatformService, c Config) http.Handler {
	g := &Gateway{client: client, config: c, routes: map[string]route{}}
	g.register()
	return g
}
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	origin := r.Header.Get("Origin")
	if origin != "" {
		allowed := false
		for _, v := range g.config.CORSOrigins {
			if origin == v {
				allowed = true
				break
			}
		}
		if !allowed {
			writeError(w, fault.Forbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition, Content-Length")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, access-token, tenant-id, app-id, unit-id, section-id, X-Request-ID")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if (r.URL.Path == "/app/file/upload" || r.URL.Path == "/app/file/limits") && g.config.Upload != nil {
		g.config.Upload.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/app/file/download" && g.config.Download != nil {
		g.config.Download.ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/files/") && g.config.Files != nil {
		g.config.Files.ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/apps/") && g.config.AppProxy != nil {
		g.config.AppProxy.ServeHTTP(w, r)
		return
	}
	rt, ok := g.routes[r.URL.Path]
	if !ok {
		writeError(w, fault.NotFound)
		return
	}
	if r.Method != rt.method {
		writeError(w, fault.New(405, "请求方法不支持"))
		return
	}
	in, e := readPayload(w, r)
	if e != nil {
		writeError(w, e)
		return
	}
	actor, e := RequestContext(r)
	if e != nil {
		writeError(w, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	ctx = rpcauth.WithCredential(ctx, g.config.GatewayKey)
	if rt.check {
		auth, authErr := g.client.Auth(ctx, actor)
		if authErr != nil {
			writeError(w, authErr)
			return
		}
		// Tenant role codes are labels, never proof of platform privilege.
		if !auth.GetPlatformAdmin() {
			_, e = g.client.CheckAccess(ctx, &pb.AccessRequest{Context: actor, AppCode: "basic", Method: r.Method, Path: r.URL.Path})
			if e != nil {
				writeError(w, e)
				return
			}
		}
	}
	result, e := rt.call(ctx, actor, in)
	if e != nil {
		writeError(w, e)
		return
	}
	WriteSuccess(w, result)
}
func RequestContext(r *http.Request) (*pb.Context, error) {
	c := &pb.Context{Token: r.Header.Get("access-token")}
	for k, dst := range map[string]*int64{"tenant-id": &c.TenantId, "app-id": &c.AppId, "unit-id": &c.UnitId, "section-id": &c.SectionId} {
		if v := r.Header.Get(k); v != "" {
			n, e := strconv.ParseInt(v, 10, 64)
			if e != nil || n < 0 {
				return nil, fault.Invalid("上下文标识格式错误")
			}
			*dst = n
		}
	}
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return nil, e
	}
	c.RequestId = hex.EncodeToString(b)
	return c, nil
}
func WriteSuccess(w http.ResponseWriter, data any) {
	writeJSON(w, map[string]any{"code": 0, "data": data, "msg": "success", "mesc": ""})
}
func writeError(w http.ResponseWriter, e error) {
	code, msg := fault.Code(e)
	var me *microerrors.Error
	if errors.As(e, &me) && me.Code >= 400 && me.Code < 500 {
		code, msg = me.Code, me.Detail
	}
	writeJSON(w, map[string]any{"code": code, "data": nil, "msg": msg, "mesc": msg})
}
func WriteError(w http.ResponseWriter, e error) { writeError(w, e) }
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func readPayload(w http.ResponseWriter, r *http.Request) (payload, error) {
	v := payload{}
	for k, a := range r.URL.Query() {
		if len(a) > 1 || strings.HasSuffix(k, "[]") {
			v[strings.TrimSuffix(k, "[]")] = a
		} else {
			v[k] = a[0]
		}
	}
	if r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		d := json.NewDecoder(r.Body)
		d.UseNumber()
		var body payload
		if e := d.Decode(&body); e != nil || body == nil {
			return nil, fault.Invalid("请求必须为有效JSON对象")
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			return nil, fault.Invalid("请求内容不合法")
		}
		for k, x := range body {
			v[k] = x
		}
	}
	if e := validateNumbers(v); e != nil {
		return nil, e
	}
	return v, nil
}
func (p payload) str(keys ...string) string {
	for _, k := range keys {
		if v, ok := p[k]; ok {
			switch x := v.(type) {
			case string:
				return x
			case json.Number:
				return string(x)
			}
		}
	}
	return ""
}
func (p payload) num(keys ...string) int64 {
	for _, k := range keys {
		if v, ok := p[k]; ok {
			switch x := v.(type) {
			case float64:
				return int64(x)
			case int64:
				return x
			case int:
				return int64(x)
			case json.Number:
				n, _ := x.Int64()
				return n
			case string:
				n, _ := strconv.ParseInt(x, 10, 64)
				return n
			}
		}
	}
	return 0
}
func (p payload) arr(key string) []int64 {
	out := []int64{}
	switch v := p[key].(type) {
	case []any:
		for _, x := range v {
			out = append(out, payload{"v": x}.num("v"))
		}
	case []string:
		for _, x := range v {
			out = append(out, payload{"v": x}.num("v"))
		}
	}
	return out
}
func (p payload) obj(key string) payload {
	if v, ok := p[key].(map[string]any); ok {
		return payload(v)
	}
	return payload{}
}
func (p payload) boolean(key string) bool {
	v := p[key]
	return v == true || p.num(key) == 1 || p.str(key) == "true"
}
func (p payload) decode(out any) error {
	b, e := json.Marshal(p)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(b, out); e != nil {
		return fault.Invalid("字段类型错误")
	}
	return nil
}
func normalize(p payload) payload {
	out := payload{}
	for k, v := range p {
		out[k] = v
		if k == "id" || strings.HasSuffix(k, "_id") || k == "status" || k == "sort" || k == "sex" || k == "expires_at" {
			if _, ok := v.(string); ok {
				out[k] = p.num(k)
			}
		}
		if k == "is_public" || k == "is_data_access" {
			out[k] = p.boolean(k)
		}
	}
	return out
}
func empty[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
func object(v any) payload {
	b, _ := json.Marshal(v)
	out := payload{}
	_ = json.Unmarshal(b, &out)
	return out
}

func userObject(user *pb.User) payload {
	out := object(user)
	if out == nil {
		return nil
	}
	// Zero is a valid "not set" value needed by the reused edit forms.
	out["sex"] = user.GetSex()
	out["created_at"] = user.GetCreatedAt()
	out["updated_at"] = user.GetUpdatedAt()
	return out
}
func queryContext(c *pb.Context, p payload, kind pb.Kind) *pb.QueryRequest {
	q := &pb.QueryRequest{Context: c, Kind: kind, Id: p.num("id"), TargetTenantId: p.num("tenant_id"), TargetAppId: p.num("app_id"), ParentId: p.num("parent_id"), RoleId: p.num("role_id"), Page: int32(p.num("page")), PageSize: int32(p.num("pageSize", "page_size")), Sort: p.str("field", "sort"), Desc: strings.HasPrefix(p.str("order"), "desc"), OrgId: p.num("org_id"), IncludeChildren: p.boolean("with_child")}
	q.Code = strings.TrimSpace(p.str("code"))
	q.TenantName = strings.TrimSpace(p.str("tenant_name"))
	q.AppName = strings.TrimSpace(p.str("app_name"))
	searchKeys := []string{"keyword", "name", "search"}
	if kind == pb.Kind_USERS || kind == pb.Kind_MEMBERS {
		q.Name = strings.TrimSpace(p.str("name"))
		q.Phone = strings.TrimSpace(p.str("phone"))
		searchKeys = []string{"keyword", "search"}
	}
	for _, key := range searchKeys {
		if value := strings.TrimSpace(p.str(key)); value != "" {
			q.Search = value
			break
		}
	}
	return q
}

// Reject malformed IDs and state values rather than accidentally interpreting them as zero.
func validateNumbers(p payload) error {
	integer := func(v any) bool {
		switch x := v.(type) {
		case string:
			if x == "" {
				return true
			}
			_, e := strconv.ParseInt(x, 10, 64)
			return e == nil
		case json.Number:
			_, e := x.Int64()
			return e == nil
		case float64:
			return x == float64(int64(x))
		case int, int64:
			return true
		}
		return false
	}
	for k, v := range p {
		if k == "id" || strings.HasSuffix(k, "_id") || k == "status" || k == "sex" || k == "page" || k == "pageSize" || k == "page_size" || k == "verify_status" {
			if !integer(v) {
				return fault.Invalid("整数参数格式错误: " + k)
			}
		}
		if strings.HasSuffix(k, "_ids") {
			switch values := v.(type) {
			case []any:
				for _, id := range values {
					if !integer(id) {
						return fault.Invalid("ID列表格式错误")
					}
				}
			case []string:
				for _, id := range values {
					if !integer(id) {
						return fault.Invalid("ID列表格式错误")
					}
				}
			default:
				return fault.Invalid("ID列表必须为数组")
			}
		}
	}
	return nil
}
