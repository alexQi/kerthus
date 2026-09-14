package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-micro.dev/v6/client"
	pb "kerthus/gen/go/saas/v1"
	agentcore "kerthus/internal/agent"
	"kerthus/internal/saas/domain/fault"
)

func TestAgentToolNameUsesOpenAICompatibleCharacters(t *testing.T) {
	got := agentToolName("/system:tenant/detail:employee/create")
	if got != "http_system_tenant_detail_employee_create" {
		t.Fatalf("agentToolName() = %q", got)
	}
	for _, r := range got {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			t.Fatalf("tool name contains unsupported character %q", r)
		}
	}
}

func TestProviderURLRejectsPrivateTargets(t *testing.T) {
	for _, endpoint := range []string{"http://127.0.0.1:8080", "http://169.254.169.254", "http://10.0.0.2", "http://localhost:8080", "file:///tmp/secrets"} {
		if _, err := providerURL(endpoint, "/models"); err == nil {
			t.Fatalf("private provider endpoint accepted: %s", endpoint)
		}
	}
	if u, err := providerURL("https://api.example.com/v1", "/models"); err != nil || u.String() != "https://api.example.com/v1/models" {
		t.Fatalf("public provider endpoint rejected or normalized incorrectly: %v %v", u, err)
	}
}

func TestAgentRouteRiskTreatsDestructiveGETAsConfirmation(t *testing.T) {
	for _, path := range []string{"/system/tenant/delete", "/system/tenant/setStatus", "/system/role/authRoleResource"} {
		_, confirm := agentRouteRisk(path, http.MethodGet)
		if !confirm {
			t.Fatalf("destructive route %s did not require confirmation", path)
		}
	}
}

type agentProviderAuthGate struct{ mockPlatform }

func (m *agentProviderAuthGate) Auth(ctx context.Context, c *pb.Context, _ ...client.CallOption) (*pb.AuthReply, error) {
	if c.GetToken() == "" {
		return nil, fault.Unauthorized
	}
	if c.GetTenantId() != 9 {
		return nil, fault.Forbidden
	}
	return &pb.AuthReply{PlatformAdmin: true}, nil
}

func TestAgentProviderTestRequiresAuthenticatedTenant(t *testing.T) {
	h := New(&agentProviderAuthGate{}, Config{})
	r := httptest.NewRequest(http.MethodPost, "/api/agent/provider/test", strings.NewReader(`{"endpoint":"http://127.0.0.1:1","model":"test"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out payload
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.num("code"); got != 401 {
		t.Fatalf("anonymous provider test returned code %d, want 401", got)
	}
}

func TestAgentProviderModelsRejectsCrossTenantContext(t *testing.T) {
	h := New(&agentProviderAuthGate{}, Config{})
	r := httptest.NewRequest(http.MethodGet, "/api/agent/provider/models", nil)
	r.Header.Set("access-token", "token")
	r.Header.Set("tenant-id", "10")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out payload
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.num("code"); got != 403 {
		t.Fatalf("cross-tenant provider lookup returned code %d, want 403", got)
	}
}

func TestMaskTenantProvidersRemovesCredentials(t *testing.T) {
	raw := `[{"provider":"openai","api_key":"secret-value","model":"gpt"},{"provider":"other","apiKey":"another-secret"}]`
	masked := maskTenantProviders(raw)
	if strings.Contains(masked, "secret-value") || strings.Contains(masked, "another-secret") {
		t.Fatalf("provider credentials leaked in masked JSON: %s", masked)
	}
	if !strings.Contains(masked, `"api_key":"********"`) || !strings.Contains(masked, `"apiKey":"********"`) {
		t.Fatalf("masked JSON did not retain credential placeholders: %s", masked)
	}
}

type providerTenantQueryPlatform struct{ mockPlatform }

func (m *providerTenantQueryPlatform) Query(ctx context.Context, q *pb.QueryRequest, _ ...client.CallOption) (*pb.QueryReply, error) {
	return &pb.QueryReply{Total: 1, Tenants: []*pb.Tenant{{Id: 9, Name: "tenant", AgentProviders: `[{"provider":"openai","api_key":"secret-value"}]`}}}, nil
}

func TestTenantQueryMasksProviderCredentials(t *testing.T) {
	h := New(&providerTenantQueryPlatform{}, Config{})
	out := request(t, h, http.MethodGet, "/system/tenant/query", "")
	items, ok := out.obj("data")["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected tenant query response: %#v", out)
	}
	row := object(items[0])
	providers := row.str("agent_providers")
	if strings.Contains(providers, "secret-value") || !strings.Contains(providers, "********") {
		t.Fatalf("tenant provider credentials leaked: %s", providers)
	}
}

func TestAgentRouteRiskRequiresConfirmationForDestructiveRoutes(t *testing.T) {
	tests := []struct {
		path string
		want agentcore.RiskLevel
	}{
		{"/system/tenant/delete", agentcore.RiskDelete},
		{"/system/tenant/detail/apps/deauth", agentcore.RiskDelete},
		{"/system/employee/resetPassword", agentcore.RiskDelete},
		{"/system/tenant/approve", agentcore.RiskPermission},
	}
	for _, tt := range tests {
		got, confirm := agentRouteRisk(tt.path, http.MethodGet)
		if got != tt.want || !confirm {
			t.Fatalf("agentRouteRisk(%q) = (%q, %v), want (%q, true)", tt.path, got, confirm, tt.want)
		}
	}
}

func TestResolveAgentProviderConfigUsesStoredCredentialForMaskedInput(t *testing.T) {
	tenant := &pb.Tenant{AgentProviders: `[{"code":"openai-main","provider":"openai","model":"gpt","endpoint":"https://provider.example","api_key":"secret-value"}]`}
	provider, model, endpoint, key, err := resolveAgentProviderConfig(tenant, payload{"code": "openai-main", "api_key": "********"})
	if err != nil {
		t.Fatal(err)
	}
	if provider != "openai" || model != "gpt" || endpoint != "https://provider.example" || key != "secret-value" {
		t.Fatalf("stored provider config was not resolved: %q %q %q %q", provider, model, endpoint, key)
	}
}

func TestResolveAgentProviderConfigRequiresKeyWhenNoStoredMatch(t *testing.T) {
	tenant := &pb.Tenant{AgentProviders: `[{"code":"openai-main","model":"gpt","endpoint":"https://provider.example","api_key":"secret-value"}]`}
	_, _, _, _, err := resolveAgentProviderConfig(tenant, payload{"code": "different", "model": "gpt", "endpoint": "https://provider.example", "api_key": "********"})
	if err == nil || !strings.Contains(err.Error(), "API Key") {
		t.Fatalf("missing provider key was not rejected explicitly: %v", err)
	}
}

type platformTenantListAgentPlatform struct {
	mockPlatform
	appCode string
	query   []*pb.QueryRequest
}

func (m *platformTenantListAgentPlatform) Auth(ctx context.Context, c *pb.Context, _ ...client.CallOption) (*pb.AuthReply, error) {
	return &pb.AuthReply{PlatformAdmin: true, Context: &pb.LoginReply{UserId: 7, TenantId: c.GetTenantId(), AppId: c.GetAppId()}}, nil
}

func (m *platformTenantListAgentPlatform) Query(ctx context.Context, q *pb.QueryRequest, _ ...client.CallOption) (*pb.QueryReply, error) {
	m.query = append(m.query, q)
	if q.GetKind() == pb.Kind_APPS {
		return &pb.QueryReply{Apps: []*pb.App{{Id: q.GetId(), Code: m.appCode}}}, nil
	}
	return &pb.QueryReply{Total: 2, Tenants: []*pb.Tenant{{Id: 1, Name: "租户一"}, {Id: 2, Name: "租户二"}}}, nil
}

func TestPlatformTenantListToolQueriesAllTenants(t *testing.T) {
	m := &platformTenantListAgentPlatform{appCode: "system"}
	g := &Gateway{client: m}
	actor := &pb.Context{TenantId: 1, AppId: 9}
	auth := &pb.AuthReply{PlatformAdmin: true, Context: &pb.LoginReply{UserId: 7, TenantId: 1, AppId: 9}}
	tools := g.agentToolOptions(actor, auth, httptest.NewRequest(http.MethodPost, "/api/agent/sessions/x/messages", nil))
	var list agentcore.Tool
	for _, item := range tools {
		if item.Definition.Name == "tenant_list" {
			list = item
			break
		}
	}
	if list.Handler == nil {
		t.Fatal("platform tenant_list tool was not exposed for system app")
	}
	raw, err := list.Handler(context.Background(), map[string]any{"payload": map[string]any{"page": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "租户一") || !strings.Contains(raw, "租户二") {
		t.Fatalf("tenant_list did not return all tenants: %s", raw)
	}
	if len(m.query) < 2 || m.query[len(m.query)-1].GetKind() != pb.Kind_TENANTS || m.query[len(m.query)-1].GetId() != 0 || m.query[len(m.query)-1].GetTargetTenantId() != 0 || m.query[len(m.query)-1].GetIncludeAgentCredentials() {
		t.Fatalf("tenant_list query was not platform-wide and credential-safe: %#v", m.query)
	}
}

type appScopedAgentPlatform struct {
	platformTenantListAgentPlatform
	checks []*pb.AccessRequest
}

func (m *appScopedAgentPlatform) CheckAccess(ctx context.Context, req *pb.AccessRequest, _ ...client.CallOption) (*pb.AccessReply, error) {
	m.checks = append(m.checks, req)
	if req.GetAppCode() != "system" || req.GetPath() == "/system/denied" {
		return nil, fault.Forbidden
	}
	return &pb.AccessReply{}, nil
}

func TestAgentRouteToolsAreFilteredByCurrentAppAccess(t *testing.T) {
	m := &appScopedAgentPlatform{platformTenantListAgentPlatform: platformTenantListAgentPlatform{appCode: "system"}}
	g := &Gateway{client: m, routes: map[string]route{
		"/system/allowed": {method: http.MethodGet, check: true},
		"/system/denied":  {method: http.MethodGet, check: true},
	}}
	auth := &pb.AuthReply{PlatformAdmin: true, Context: &pb.LoginReply{UserId: 7, TenantId: 1, AppId: 9}}
	tools := g.agentToolOptions(&pb.Context{TenantId: 1, AppId: 9}, auth, httptest.NewRequest(http.MethodPost, "/api/agent/sessions/x/messages", nil))
	seen := map[string]bool{}
	for _, item := range tools {
		seen[item.Definition.Name] = true
	}
	if !seen[agentToolName("/system/allowed")] || seen[agentToolName("/system/denied")] {
		t.Fatalf("route tools were not filtered by current app access: %#v", seen)
	}
	for _, req := range m.checks {
		if req.GetAppCode() != "system" {
			t.Fatalf("tool access check used the wrong app: %#v", req)
		}
	}
}

type providerToolsPlatform struct{ mockPlatform }

func (m *providerToolsPlatform) Query(ctx context.Context, q *pb.QueryRequest, _ ...client.CallOption) (*pb.QueryReply, error) {
	if q.GetKind() == pb.Kind_APPS {
		return &pb.QueryReply{Apps: []*pb.App{{Id: q.GetId(), Code: "basic"}}}, nil
	}
	return &pb.QueryReply{Total: 1, Tenants: []*pb.Tenant{{Id: 9, AgentProviders: `[{"code":"main","name":"主 Provider","provider":"openai","model":"gpt-test","endpoint":"https://provider.example/v1","api_key":"secret-value","models":[{"id":"gpt-test","name":"GPT Test"}],"enabled":true,"default":true}]`}}}, nil
}

func TestBasicAgentInjectsProviderToolsForProviderPermission(t *testing.T) {
	g := &Gateway{client: &providerToolsPlatform{}}
	a := &pb.Context{TenantId: 9, AppId: 2}
	auth := &pb.AuthReply{Permissions: []string{"basic:system:provider"}, Context: &pb.LoginReply{UserId: 7, TenantId: 9, AppId: 2}}
	tools := g.agentToolOptions(a, auth, httptest.NewRequest(http.MethodPost, "/api/agent/sessions/x/messages", nil))
	seen := map[string]agentcore.Tool{}
	for _, tool := range tools {
		seen[tool.Definition.Name] = tool
	}
	for _, name := range []string{"provider_list", "provider_models", "provider_test"} {
		if seen[name].Handler == nil {
			t.Fatalf("provider tool %q was not injected", name)
		}
	}
	raw, err := seen["provider_list"].Handler(context.Background(), map[string]any{"payload": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "secret-value") || !strings.Contains(raw, "gpt-test") {
		t.Fatalf("provider tool returned unsafe or incomplete data: %s", raw)
	}
}

func TestBasicAgentDoesNotInjectProviderToolsWithoutPermission(t *testing.T) {
	g := &Gateway{client: &providerToolsPlatform{}}
	a := &pb.Context{TenantId: 9, AppId: 2}
	auth := &pb.AuthReply{Permissions: []string{"basic:system:dashboard"}, Context: &pb.LoginReply{UserId: 7, TenantId: 9, AppId: 2}}
	for _, tool := range g.agentToolOptions(a, auth, httptest.NewRequest(http.MethodPost, "/api/agent/sessions/x/messages", nil)) {
		if strings.HasPrefix(tool.Definition.Name, "provider_") {
			t.Fatalf("provider tool %q was injected without provider permission", tool.Definition.Name)
		}
	}
}

func TestBasicAgentToolManifestIncludesProviderCapabilities(t *testing.T) {
	g := &Gateway{client: &providerToolsPlatform{}}
	a := &pb.Context{TenantId: 9, AppId: 2}
	auth := &pb.AuthReply{Permissions: []string{"basic:system:provider"}, Context: &pb.LoginReply{UserId: 7, TenantId: 9, AppId: 2}}
	manifests := g.availableAgentTools(context.Background(), a, auth)
	seen := map[string]bool{}
	for _, item := range manifests {
		seen[item.ID] = true
	}
	for _, id := range []string{"provider.list", "provider.models", "provider.test"} {
		if !seen[id] {
			t.Fatalf("provider manifest %q was not exposed", id)
		}
	}
}
