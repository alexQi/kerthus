package gateway

import (
	"context"
	"encoding/json"
	"go-micro.dev/v6/client"
	microerrors "go-micro.dev/v6/errors"
	"go-micro.dev/v6/metadata"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/platform/rpcauth"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type mockPlatform struct {
	pb.PlatformService
	save   *pb.SaveRequest
	query  *pb.QueryRequest
	grants *pb.RoleResourcesRequest
	check  *pb.AccessRequest
	admin  bool
}

type roleCollisionPlatform struct{ mockPlatform }

func (m *roleCollisionPlatform) CheckAccess(ctx context.Context, r *pb.AccessRequest, opts ...client.CallOption) (*pb.AccessReply, error) {
	m.check = r
	return nil, microerrors.Forbidden("test", "exact operation is not granted")
}

func TestTenantAdminRoleNameCannotBypassOperationAuthorization(t *testing.T) {
	m := &roleCollisionPlatform{mockPlatform: mockPlatform{admin: true}}
	out := request(t, New(m, Config{}), "GET", "/system/org/query", "")
	if out.num("code") != 403 || m.check == nil || m.query != nil {
		t.Fatal("tenant role named admin bypassed exact operation authorization")
	}
}

type explicitPlatform struct{ roleCollisionPlatform }

func (m *explicitPlatform) Auth(ctx context.Context, c *pb.Context, opts ...client.CallOption) (*pb.AuthReply, error) {
	r, e := m.mockPlatform.Auth(ctx, c, opts...)
	r.PlatformAdmin = true
	return r, e
}

func TestExplicitPlatformIdentityRetainsCrossTenantManagement(t *testing.T) {
	m := &explicitPlatform{}
	out := request(t, New(m, Config{}), "GET", "/system/org/query?tenant_id=12", "")
	if out.num("code") != 0 || m.check != nil || m.query == nil || m.query.TargetTenantId != 12 {
		t.Fatal("explicit platform identity lost cross-tenant management")
	}
}

func (m *mockPlatform) Auth(ctx context.Context, c *pb.Context, opts ...client.CallOption) (*pb.AuthReply, error) {
	roles := []string{"tenant-admin"}
	if m.admin {
		roles = []string{"admin"}
	}
	return &pb.AuthReply{Roles: roles, Resources: []*pb.Resource{{Id: 1, Code: "basic:root", Name: "基础管理", Type: "menu", Path: "/basic", Component: "LAYOUT"}, {Id: 2, ParentId: 1, Code: "basic:home", Name: "工作台", Type: "view", Path: "/basic/dashboard", Component: "common/dashboard/index"}, {Id: 3, ParentId: 1, Code: "basic:save", Name: "保存", Type: "action"}}}, nil
}
func (m *mockPlatform) CheckAccess(ctx context.Context, r *pb.AccessRequest, opts ...client.CallOption) (*pb.AccessReply, error) {
	m.check = r
	return &pb.AccessReply{}, nil
}
func (m *mockPlatform) Query(ctx context.Context, r *pb.QueryRequest, opts ...client.CallOption) (*pb.QueryReply, error) {
	m.query = r
	return &pb.QueryReply{Total: 2, Orgs: []*pb.Org{{Id: 1, Name: "公司", Type: "unit"}, {Id: 2, ParentId: 1, Name: "部门", Type: "department"}}, Apps: []*pb.App{{Id: 5, Name: "基础管理", Code: "basic"}}}, nil
}
func (m *mockPlatform) Save(ctx context.Context, r *pb.SaveRequest, opts ...client.CallOption) (*pb.SaveReply, error) {
	m.save = r
	return &pb.SaveReply{Id: 42}, nil
}
func (m *mockPlatform) SetRoleResources(ctx context.Context, r *pb.RoleResourcesRequest, opts ...client.CallOption) (*pb.Result, error) {
	m.grants = r
	return &pb.Result{Success: true}, nil
}
func (m *mockPlatform) Login(ctx context.Context, r *pb.LoginRequest, opts ...client.CallOption) (*pb.LoginReply, error) {
	if key, _ := metadata.Get(ctx, rpcauth.CredentialHeader); key != "test-service-key" {
		panic("gateway principal missing")
	}
	return &pb.LoginReply{UserId: 7, AccessToken: "opaque", TenantId: 9, AppId: 5, AppCode: "basic"}, nil
}
func request(t *testing.T, h http.Handler, method, path, body string) payload {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("tenant-id", "9")
	r.Header.Set("app-id", "5")
	r.Header.Set("access-token", "token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out payload
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestLegacyLoginAndMenus(t *testing.T) {
	m := &mockPlatform{}
	h := New(m, Config{GatewayKey: "test-service-key"})
	out := request(t, h, "POST", "/system/user/login", `{"username":"user","password":"secret"}`)
	if out.num("code") != 0 || out.obj("data").num("seat_id") != 0 || out.obj("data").str("app_code") != "basic" {
		t.Fatalf("bad login DTO: %#v", out)
	}
	out = request(t, h, "GET", "/system/user/auth", "")
	data := out.obj("data")
	routes := data["routes"].([]any)
	menus := data["menus"].([]any)
	if object(routes[0]).str("name") != "basic_root" || object(menus[0]).str("name") != "基础管理" {
		t.Fatal("route names leaked into menu titles")
	}
	if len(object(routes[0])["children"].([]any)) != 1 {
		t.Fatal("action became navigable")
	}
}
func TestOrgTreeAndContext(t *testing.T) {
	m := &mockPlatform{}
	h := New(m, Config{})
	out := request(t, h, "GET", "/system/org/query?tenant_id=12", "")
	tree := out["data"].([]any)
	if len(tree) != 1 || object(tree[0]).str("title") != "公司" || len(object(tree[0])["children"].([]any)) != 1 {
		t.Fatalf("bad tree %#v", tree)
	}
	if m.query.Context.TenantId != 9 || m.query.TargetTenantId != 12 || m.check.Path != "/system/org/query" {
		t.Fatal("actor and target context mixed")
	}
}
func TestMemberSaveAndRoleScope(t *testing.T) {
	m := &mockPlatform{}
	h := New(m, Config{})
	out := request(t, h, "POST", "/system/employee/save", `{"id":"7","name":"Alice","tenant_id":"12","app_id":"5","org_ids":[2],"position_ids":[],"status":0}`)
	if out.num("code") != 0 || m.save.GetMember().User.Id != 7 || m.save.GetMember().TenantId != 12 || m.save.GetMember().Status != nil {
		t.Fatalf("member DTO/state boundary broken %#v", out)
	}
	request(t, h, "POST", "/system/role/authRoleResource", `{"role_id":4,"resource_map":{"5":{"ids":[8,9],"scope":{"8":0}}}}`)
	if len(m.grants.Grants) != 2 || m.grants.Grants[0].DataScope != 0 || m.grants.Grants[1].DataScope != 5 {
		t.Fatal("explicit all scope or safe omitted scope lost")
	}
}
func TestCORSAndInputBoundaries(t *testing.T) {
	h := New(&mockPlatform{}, Config{CORSOrigins: []string{"http://localhost:15173"}})
	r := httptest.NewRequest("POST", "/system/user/login", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://attacker.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out payload
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.num("code") != 403 || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unexpected cross-origin access")
	}
	for _, body := range []string{`{} {}`, `[]`, `null`, `{"status":"oops"}`, `{"tenant_id":1.5}`} {
		out := request(t, h, "POST", "/system/employee/save", body)
		if out.num("code") != 400 {
			t.Fatalf("accepted malformed object %s", body)
		}
	}
	r = httptest.NewRequest("GET", "/system/user/auth", nil)
	r.Header.Set("tenant-id", "NaN")
	if _, e := RequestContext(r); e == nil {
		t.Fatal("invalid context accepted")
	}
}

type pagedPlatform struct {
	pb.PlatformService
	pages []int32
}

func (m *pagedPlatform) Query(ctx context.Context, q *pb.QueryRequest, opts ...client.CallOption) (*pb.QueryReply, error) {
	m.pages = append(m.pages, q.Page)
	r := &pb.QueryReply{Total: 1001}
	if q.Page == 1 {
		for i := int64(1); i <= 1000; i++ {
			r.Resources = append(r.Resources, &pb.Resource{Id: i})
		}
	} else if q.Page == 2 {
		r.Resources = append(r.Resources, &pb.Resource{Id: 1001})
	}
	return r, nil
}
func TestFullResourceEditorLoadsAllPages(t *testing.T) {
	m := &pagedPlatform{}
	g := &Gateway{client: m}
	in := &pb.QueryRequest{Kind: pb.Kind_RESOURCES, Page: 3, PageSize: 10}
	out, e := g.queryAll(context.Background(), in)
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Resources) != 1001 || len(m.pages) != 2 || out.Resources[1000].Id != 1001 || in.Page != 3 {
		t.Fatal("resource replacement input was truncated or caller query mutated")
	}
}
func TestCompatibilityContractRoutesExist(t *testing.T) {
	b, e := os.ReadFile("../../api/http/legacy.openapi.yaml")
	if e != nil {
		t.Fatal(e)
	}
	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if e = json.Unmarshal(b, &spec); e != nil {
		t.Fatal(e)
	}
	dispatched := false
	fileHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { dispatched = true; w.WriteHeader(204) })
	g := New(&mockPlatform{}, Config{Upload: fileHandler, Download: fileHandler}).(*Gateway)
	for path, methods := range spec.Paths {
		if path == "/app/file/upload" || path == "/app/file/download" || path == "/app/file/limits" {
			for method := range methods {
				dispatched = false
				g.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(strings.ToUpper(method), path, nil))
				if !dispatched {
					t.Errorf("file contract route has no gateway adapter: %s %s", method, path)
				}
			}
			continue
		}
		rt, ok := g.routes[path]
		if !ok {
			t.Errorf("contract route has no gateway adapter: %s", path)
			continue
		}
		for method := range methods {
			if strings.ToUpper(method) != rt.method {
				t.Errorf("method mismatch for %s: %s", path, method)
			}
		}
	}
}

type tenantPlatform struct{ mockPlatform }

func (m *tenantPlatform) Query(ctx context.Context, q *pb.QueryRequest, opts ...client.CallOption) (*pb.QueryReply, error) {
	return &pb.QueryReply{Total: 1, Tenants: []*pb.Tenant{{Id: 12, Name: "测试租户", ContactPerson: "张三", ContactPhone: "13800138000", ContactEmail: "contact@example.test", ExpiresAt: 0}}}, nil
}
func TestTenantLegacyFieldsAndExpiry(t *testing.T) {
	m := &tenantPlatform{}
	h := New(m, Config{})
	out := request(t, h, "GET", "/system/tenant/query", "")
	items := out.obj("data")["items"].([]any)
	row := object(items[0])
	if row.str("register_type") != "create" || row.str("contact_phone") != "13800138000" || row.str("contact_email") != "contact@example.test" || row.num("expiration_time") != 0 {
		t.Fatalf("tenant response incompatible: %#v", row)
	}
	for _, tt := range []struct {
		expiry string
		want   int64
	}{{`"2027-01-02 03:04:05"`, 1798830245}, {`"1798830245"`, 1798830245}, {`0`, 0}} {
		out = request(t, h, "POST", "/system/tenant/save", `{"id":"12","name":"测试租户","contact_person":"张三","contact_phone":"13800138000","contact_email":"contact@example.test","expiration_time":`+tt.expiry+`}`)
		if out.num("code") != 0 {
			t.Fatalf("save expiry %s failed: %#v", tt.expiry, out)
		}
		tenant := m.save.GetTenant()
		if tenant.ExpiresAt != tt.want || tenant.ContactPhone != "13800138000" || tenant.ContactEmail != "contact@example.test" {
			t.Fatalf("lost fields for %s: %#v", tt.expiry, tenant)
		}
	}
	out = request(t, h, "POST", "/system/tenant/save", `{"name":"测试租户","expiration_time":"not-a-date"}`)
	if out.num("code") != 400 {
		t.Fatal("malformed date silently became no expiry")
	}
}

type appPlatform struct{ mockPlatform }

func (m *appPlatform) Query(ctx context.Context, q *pb.QueryRequest, opts ...client.CallOption) (*pb.QueryReply, error) {
	r := &pb.QueryReply{Total: 1}
	if q.Kind == pb.Kind_RESOURCES {
		r.Resources = []*pb.Resource{{Id: 7, AppId: 5, Name: "工作台", Code: "basic:home", Type: "view"}}
	} else {
		r.Apps = []*pb.App{{Id: 5, Name: "基础管理", Code: "basic", ExpiresAt: 123}}
	}
	return r, nil
}
func TestAppDTO(t *testing.T) {
	m := &appPlatform{}
	h := New(m, Config{})
	for _, path := range []string{"/system/app/query", "/system/employee/queryTenantApps", "/system/employee/getTenantApps", "/system/app/getGlobalResource", "/system/app/getTenantResources"} {
		out := request(t, h, "GET", path, "")
		if out.num("code") != 0 {
			t.Fatalf("app query failed %s: %#v", path, out)
		}
		var row payload
		switch path {
		case "/system/employee/getTenantApps":
			row = object(out["data"].([]any)[0])
		case "/system/app/getGlobalResource", "/system/app/getTenantResources":
			row = out.obj("data").obj("5")
		default:
			row = object(out.obj("data")["items"].([]any)[0])
		}
		if row.str("type") != "self" || row["is_public"] != false || row.num("expiration_time") != 123 {
			t.Fatalf("incompatible app DTO %s: %#v", path, row)
		}
	}
	for _, body := range []string{`{"name":"应用","code":"notes","type":"third","url":"https://example.com"}`, `{"name":"应用","code":"notes","is_public":true}`} {
		out := request(t, h, "POST", "/system/app/save", body)
		if out.num("code") != 0 {
			t.Fatal("compatible app mode did not reach core validation")
		}
		if body == `{"name":"应用","code":"notes","is_public":true}` && !m.save.GetApp().IsPublic {
			t.Fatal("publication metadata lost")
		}
	}
}
