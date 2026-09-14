package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	microagent "go-micro.dev/v6/agent"
	"go-micro.dev/v6/ai"
	pb "kerthus/gen/go/saas/v1"
	agentcore "kerthus/internal/agent"
	"kerthus/internal/platform/rpcauth"
	"kerthus/internal/saas/domain/fault"
)

// handleAgent owns the HTTP boundary for the page-aware agent. It deliberately
// keeps model execution separate until a provider is configured and a policy
// backed tool registry is attached.
func (g *Gateway) handleAgent(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/api/agent/provider/models" {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			writeError(w, fault.New(405, "请求方法不支持"))
			return true
		}
		g.agentProviderModels(w, r)
		return true
	}
	if r.URL.Path == "/api/agent/provider/test" {
		g.agentProviderTest(w, r)
		return true
	}
	if r.URL.Path == "/api/agent/tools" {
		if r.Method != http.MethodGet {
			writeError(w, fault.New(405, "请求方法不支持"))
			return true
		}
		actor, auth, err := g.agentAuth(r)
		if err != nil {
			writeError(w, err)
			return true
		}
		WriteSuccess(w, g.availableAgentTools(r.Context(), actor, auth))
		return true
	}
	if r.URL.Path == "/api/agent/sessions" {
		switch r.Method {
		case http.MethodGet:
			g.listAgentSessions(w, r)
		case http.MethodPost:
			g.createAgentSession(w, r)
		default:
			writeError(w, fault.New(405, "请求方法不支持"))
		}
		return true
	}
	const prefix = "/api/agent/sessions/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")
	if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodGet {
		g.getAgentSession(w, r, parts[0])
		return true
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] == "context" && r.Method == http.MethodPost {
		g.updateAgentContext(w, r, parts[0])
		return true
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] == "messages" && r.Method == http.MethodPost {
		g.streamAgent(w, r, parts[0])
		return true
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] == "events" && r.Method == http.MethodGet {
		g.streamAgent(w, r, parts[0])
		return true
	}
	writeError(w, fault.NotFound)
	return true
}

func (g *Gateway) agentProviderModels(w http.ResponseWriter, r *http.Request) {
	actor, auth, err := g.agentAuth(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !canManageAgentProvider(auth) {
		writeError(w, fault.Forbidden)
		return
	}
	ctx := rpcauth.WithCredential(r.Context(), g.config.GatewayKey)
	q, err := g.client.Query(ctx, &pb.QueryRequest{Context: actor, Kind: pb.Kind_TENANTS, Id: actor.GetTenantId(), IncludeAgentCredentials: true})
	if err != nil || len(q.GetTenants()) == 0 {
		if err != nil {
			writeError(w, err)
		} else {
			writeError(w, fault.NotFound)
		}
		return
	}
	t := q.GetTenants()[0]
	provider, endpoint, apiKey := t.GetAgentProvider(), t.GetAgentEndpoint(), t.GetAgentApiKey()
	if r.Method == http.MethodPost {
		p, e := readPayload(w, r)
		if e != nil {
			writeError(w, e)
			return
		}
		provider, _, endpoint, apiKey, e = resolveAgentProviderConfig(t, p)
		if e != nil {
			writeError(w, e)
			return
		}
	}
	if listProvider := defaultProvider(t.GetAgentProviders()); r.Method == http.MethodGet && listProvider != nil {
		provider, endpoint, apiKey = listProvider.Provider, listProvider.Endpoint, listProvider.APIKey
	}
	_ = provider
	if endpoint == "" {
		writeError(w, fault.Invalid("尚未配置 Provider Endpoint"))
		return
	}
	u, err := providerURL(endpoint, "/models")
	if err != nil || u.Scheme == "" || u.Host == "" {
		writeError(w, fault.Invalid("Provider Endpoint 格式不正确"))
		return
	}
	models, err := fetchAgentProviderModels(r.Context(), u, apiKey, g.config.AgentHTTPClient)
	if err != nil {
		writeError(w, fault.New(502, err.Error()))
		return
	}
	WriteSuccess(w, models)
}

func fetchAgentProviderModels(ctx context.Context, endpoint *url.URL, apiKey string, configured *http.Client) ([]map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, errors.New("Provider 模型接口地址不正确")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 30 * time.Second}
	if configured != nil {
		*client = *configured
		if client.Timeout == 0 {
			client.Timeout = 30 * time.Second
		}
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("无法连接 Provider 模型接口")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("Provider 模型接口返回错误")
	}
	var body struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil, errors.New("Provider 模型响应格式错误")
	}
	models := make([]map[string]string, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID == "" {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		models = append(models, map[string]string{"id": m.ID, "name": name})
	}
	return models, nil
}

func (g *Gateway) agentProviderTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, fault.New(405, "请求方法不支持"))
		return
	}
	actor, auth, err := g.agentAuth(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !canManageAgentProvider(auth) {
		writeError(w, fault.Forbidden)
		return
	}
	p, err := readPayload(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	queryCtx := rpcauth.WithCredential(r.Context(), g.config.GatewayKey)
	q, err := g.client.Query(queryCtx, &pb.QueryRequest{Context: actor, Kind: pb.Kind_TENANTS, Id: actor.GetTenantId(), IncludeAgentCredentials: true})
	if err != nil || len(q.GetTenants()) == 0 {
		if err != nil {
			writeError(w, err)
		} else {
			writeError(w, fault.NotFound)
		}
		return
	}
	provider, model, endpoint, apiKey, err := resolveAgentProviderConfig(q.GetTenants()[0], p)
	_ = provider
	if err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(model) == "" {
		writeError(w, fault.Invalid("模型不能为空"))
		return
	}
	endpoint = strings.TrimRight(endpoint, "/")
	u, err := providerURL(endpoint, "/chat/completions")
	if err != nil || u.Scheme == "" || u.Host == "" {
		writeError(w, fault.Invalid("Provider Endpoint 格式不正确"))
		return
	}
	result, err := probeAgentModel(r.Context(), &http.Client{}, u, model, apiKey)
	if err != nil {
		writeError(w, fault.New(502, err.Error()))
		return
	}
	WriteSuccess(w, result)
}

// Provider credentials may be submitted to these endpoints while configuring
// the tenant, so they must use the same authenticated tenant context as the
// rest of the gateway. Platform administrators may configure any selected
// tenant; tenant users need the provider resource permission in their current
// application. The endpoint itself never accepts a target tenant override.
func canManageAgentProvider(auth *pb.AuthReply) bool {
	if auth == nil || auth.GetPlatformAdmin() {
		return auth != nil
	}
	for _, permission := range auth.GetPermissions() {
		if permission == "basic:system:provider" || permission == "tenant.manage" || permission == "system:tenant" {
			return true
		}
	}
	return false
}

// providerURL accepts either a provider origin (https://api.example.com) or
// an OpenAI-compatible /v1 endpoint and avoids duplicating the version path.
func providerURL(endpoint, suffix string) (*url.URL, error) {
	base := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	var raw string
	if strings.HasSuffix(base, "/v1") {
		raw = base + suffix
	} else {
		raw = base + "/v1" + suffix
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("provider endpoint is not a valid HTTP URL")
	}
	if blockedProviderHost(strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))) {
		return nil, fmt.Errorf("provider endpoint targets a private network")
	}
	return u, nil
}

func blockedProviderHost(host string) bool {
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "metadata.google.internal" || host == "metadata" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.168.0.0/16", "198.18.0.0/15", "224.0.0.0/4", "::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8"} {
		_, network, err := net.ParseCIDR(raw)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func (g *Gateway) availableAgentTools(requestCtx context.Context, actor *pb.Context, auth *pb.AuthReply) []agentcore.ToolManifest {
	permissions := auth.GetPermissions()
	allowed := map[string]bool{}
	for _, p := range permissions {
		allowed[p] = true
	}
	all := []agentcore.ToolManifest{
		{ID: "page.read", Version: "1", Name: "读取当前页面", Description: "读取经过脱敏的当前页面上下文", RequiredScopes: []string{"agent.page.read"}, Risk: agentcore.RiskRead, TargetService: "gateway", TimeoutMilliseconds: 3000, Idempotent: true},
		{ID: "tenant.read", Version: "1", Name: "读取当前租户信息", Description: "读取当前租户的基础信息", RequiredScopes: []string{"tenant.read"}, Risk: agentcore.RiskRead, TargetService: "platform", TimeoutMilliseconds: 3000, Idempotent: true},
	}
	out := make([]agentcore.ToolManifest, 0, len(all))
	for _, tool := range all {
		if tool.ID == "page.read" || auth.GetPlatformAdmin() || allowed[tool.RequiredScopes[0]] || allowed["*"] || (tool.ID == "tenant.read" && (allowed["tenant.query"] || allowed["tenant.manage"])) {
			out = append(out, tool)
		}
	}
	checkCtx, cancel := context.WithTimeout(requestCtx, 5*time.Second)
	defer cancel()
	appCode := g.agentAppCode(checkCtx, actor)
	if auth.GetPlatformAdmin() && appCode == "system" {
		out = append(out, agentcore.ToolManifest{ID: "tenant.list", Version: "1", Name: "查询全部租户", Description: "查询平台中的全部租户，只返回基础信息，不包含 Provider 密钥。", Risk: agentcore.RiskRead, TargetService: "platform", TimeoutMilliseconds: 5000, Idempotent: true})
	}
	if appCode == "basic" && canManageAgentProvider(auth) {
		out = append(out,
			agentcore.ToolManifest{ID: "provider.list", Version: "1", Name: "查看模型提供商", Description: "查看当前租户的模型提供商、模型和默认状态，不返回 API 密钥。", RequiredScopes: []string{"basic:system:provider"}, Risk: agentcore.RiskRead, TargetService: "gateway", TimeoutMilliseconds: 5000, Idempotent: true},
			agentcore.ToolManifest{ID: "provider.models", Version: "1", Name: "同步提供商模型", Description: "从当前租户已配置的模型提供商同步可用模型。", RequiredScopes: []string{"basic:system:provider"}, Risk: agentcore.RiskRead, TargetService: "gateway", TimeoutMilliseconds: 30000, Idempotent: true},
			agentcore.ToolManifest{ID: "provider.test", Version: "1", Name: "测试提供商模型", Description: "测试当前租户已配置模型的连通性。", RequiredScopes: []string{"basic:system:provider"}, Risk: agentcore.RiskRead, TargetService: "gateway", TimeoutMilliseconds: 30000, Idempotent: true},
		)
	}
	for path, route := range g.routes {
		if path == "/healthz" || path == "/app/info/init" {
			continue
		}
		// Do not expose public authentication/profile endpoints as tools. For
		// protected routes, use the policy service's exact operation match rather
		// than comparing resource codes with URL paths.
		if !route.check {
			continue
		}
		if _, err := g.client.CheckAccess(rpcauth.WithCredential(checkCtx, g.config.GatewayKey), &pb.AccessRequest{Context: actor, AppCode: appCode, Method: route.method, Path: path}); err != nil {
			continue
		}
		if actor == nil {
			continue
		}
		risk, confirm := agentRouteRisk(path, route.method)
		out = append(out, agentcore.ToolManifest{ID: "http." + strings.Trim(path, "/"), Version: "1", Name: "调用 " + path, Description: "调用已通过权限校验的 SaaS 接口", RequiredScopes: []string{"route:" + path}, Risk: risk, RequiresConfirmation: confirm, TargetService: "gateway", TimeoutMilliseconds: 10000, Idempotent: route.method == http.MethodGet})
	}
	return out
}

func routePermission(permissions []string, path string) bool {
	for _, permission := range permissions {
		if permission == "*" || permission == path || permission == "route:"+path || strings.TrimPrefix(permission, "/") == strings.TrimPrefix(path, "/") {
			return true
		}
	}
	return false
}

func (g *Gateway) agentAuth(r *http.Request) (*pb.Context, *pb.AuthReply, error) {
	actor, err := RequestContext(r)
	if err != nil {
		return nil, nil, err
	}
	ctx := rpcauth.WithCredential(r.Context(), g.config.GatewayKey)
	auth, err := g.client.Auth(ctx, actor)
	if err != nil {
		return nil, nil, err
	}
	return actor, auth, nil
}

// agentAppCode resolves the catalog code for the current application. Agent
// tools must be authorized against the active app rather than the basic app.
func (g *Gateway) agentAppCode(ctx context.Context, actor *pb.Context) string {
	if actor == nil || actor.GetAppId() <= 0 {
		return "basic"
	}
	q, err := g.client.Query(rpcauth.WithCredential(ctx, g.config.GatewayKey), &pb.QueryRequest{Context: actor, Kind: pb.Kind_APPS, Id: actor.GetAppId()})
	if err == nil && q != nil && len(q.GetApps()) > 0 && strings.TrimSpace(q.GetApps()[0].GetCode()) != "" {
		return q.GetApps()[0].GetCode()
	}
	return "basic"
}

func (g *Gateway) listAgentSessions(w http.ResponseWriter, r *http.Request) {
	actor, auth, err := g.agentAuth(r)
	if err != nil {
		writeError(w, err)
		return
	}
	owner := agentcore.TrustedContext{UserID: auth.GetContext().GetUserId(), TenantID: actor.GetTenantId(), AppID: actor.GetAppId(), PermissionVersion: agentPermissionVersion(auth)}
	if owner.UserID == 0 {
		writeError(w, fault.Unauthorized)
		return
	}
	items, err := g.agentStore.List(owner)
	if err != nil {
		writeError(w, fault.New(503, "Agent 会话列表读取失败"))
		return
	}
	WriteSuccess(w, items)
}

func (g *Gateway) createAgentSession(w http.ResponseWriter, r *http.Request) {
	p, err := readPayload(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	actor, auth, err := g.agentAuth(r)
	if err != nil {
		writeError(w, err)
		return
	}
	page, err := pageContext(p["context"])
	if err != nil {
		writeError(w, fault.Invalid("页面上下文格式错误"))
		return
	}
	trusted := agentcore.TrustedContext{UserID: auth.GetContext().GetUserId(), TenantID: actor.GetTenantId(), AppID: actor.GetAppId(), PermissionVersion: agentPermissionVersion(auth)}
	if trusted.UserID == 0 {
		writeError(w, fault.Unauthorized)
		return
	}
	envelope, err := agentcore.NormalizePageContext(page, trusted)
	if err != nil {
		writeError(w, fault.Invalid(err.Error()))
		return
	}
	session, err := g.agentStore.Create(envelope)
	if err != nil {
		writeError(w, fault.New(503, "Agent 会话保存失败"))
		return
	}
	WriteSuccess(w, session)
}

func (g *Gateway) getAgentSession(w http.ResponseWriter, r *http.Request, id string) {
	s, err := g.agentStore.Get(id)
	if err != nil {
		writeError(w, fault.NotFound)
		return
	}
	if err = g.verifyAgentOwner(r, s.Trusted); err != nil {
		writeError(w, err)
		return
	}
	WriteSuccess(w, s)
}

func (g *Gateway) updateAgentContext(w http.ResponseWriter, r *http.Request, id string) {
	p, err := readPayload(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	s, err := g.agentStore.Get(id)
	if err != nil {
		writeError(w, fault.NotFound)
		return
	}
	if err = g.verifyAgentOwner(r, s.Trusted); err != nil {
		writeError(w, err)
		return
	}
	page, err := pageContext(p["context"])
	if err != nil {
		writeError(w, fault.Invalid("页面上下文格式错误"))
		return
	}
	actor, auth, err := g.agentAuth(r)
	if err != nil {
		writeError(w, err)
		return
	}
	trusted := agentcore.TrustedContext{UserID: auth.GetContext().GetUserId(), TenantID: actor.GetTenantId(), AppID: actor.GetAppId(), PermissionVersion: agentPermissionVersion(auth)}
	envelope, err := agentcore.NormalizePageContext(page, trusted)
	if err != nil {
		writeError(w, fault.Invalid(err.Error()))
		return
	}
	updated, err := g.agentStore.UpdateContext(id, envelope)
	if err != nil {
		writeError(w, err)
		return
	}
	WriteSuccess(w, updated)
}

func (g *Gateway) verifyAgentOwner(r *http.Request, trusted agentcore.TrustedContext) error {
	actor, auth, err := g.agentAuth(r)
	if err != nil {
		return err
	}
	if auth.GetContext().GetUserId() != trusted.UserID || actor.GetTenantId() != trusted.TenantID || actor.GetAppId() != trusted.AppID {
		return fault.Forbidden
	}
	return nil
}

func (g *Gateway) streamAgent(w http.ResponseWriter, r *http.Request, id string) {
	s, err := g.agentStore.Get(id)
	if err != nil {
		writeError(w, fault.NotFound)
		return
	}
	if err = g.verifyAgentOwner(r, s.Trusted); err != nil {
		writeError(w, err)
		return
	}
	message := r.URL.Query().Get("message")
	if r.Method == http.MethodPost {
		p, e := readPayload(w, r)
		if e != nil {
			writeError(w, e)
			return
		}
		message = p.str("message")
	}
	if strings.TrimSpace(message) == "" {
		writeError(w, fault.Invalid("消息不能为空"))
		return
	}
	actor, auth, err := g.agentAuth(r)
	if err != nil {
		writeError(w, err)
		return
	}
	queryCtx := rpcauth.WithCredential(r.Context(), g.config.GatewayKey)
	q, err := g.client.Query(queryCtx, &pb.QueryRequest{Context: actor, Kind: pb.Kind_TENANTS, Id: s.Trusted.TenantID, IncludeAgentCredentials: true})
	if err != nil || len(q.GetTenants()) == 0 {
		if err != nil {
			writeError(w, err)
		} else {
			writeError(w, fault.NotFound)
		}
		return
	}
	t := q.GetTenants()[0]
	if p := defaultProvider(t.GetAgentProviders()); p != nil {
		t.AgentProvider, t.AgentModel, t.AgentEndpoint, t.AgentApiKey, t.AgentEnabled = p.Provider, p.Model, p.Endpoint, p.APIKey, p.Enabled
	}
	if !t.GetAgentEnabled() || t.GetAgentProvider() == "" {
		writeError(w, fault.New(503, "当前租户尚未配置 Agent provider"))
		return
	}
	if _, err := providerURL(t.GetAgentEndpoint(), "/chat/completions"); err != nil {
		writeError(w, fault.New(503, "当前租户的 Provider 地址不安全或格式不正确"))
		return
	}
	expected := s.Trusted
	expected.PermissionVersion = agentPermissionVersion(auth)
	s, lease, err := g.agentStore.BeginTurn(id, expected)
	if err != nil {
		if errors.Is(err, agentcore.ErrSessionBusy) {
			writeError(w, fault.New(409, "该会话正在生成回复"))
		} else {
			writeError(w, fault.New(503, "Agent 会话暂不可用"))
		}
		return
	}
	defer g.agentStore.AbortTurn(id, lease)
	prompt, err := g.agentSystemPrompt(queryCtx, actor, auth, t, s.Context.Page)
	if err != nil {
		writeError(w, err)
		return
	}
	toolOptions := g.agentToolOptions(actor, auth, r)
	runtime, err := agentcore.NewRuntime(agentcore.RuntimeConfig{Provider: t.GetAgentProvider(), Model: t.GetAgentModel(), BaseURL: t.GetAgentEndpoint(), APIKey: t.GetAgentApiKey(), Prompt: prompt, HTTPClient: g.config.AgentHTTPClient}, toolOptions...)
	if err != nil {
		writeError(w, fault.New(503, err.Error()))
		return
	}
	// Bound each model request so an upstream that never closes its stream cannot
	// leave an HTTP connection and the UI in an eternal "generating" state.
	streamCtx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	stream, err := runtime.Stream(streamCtx, message, s.History)
	if err != nil {
		writeError(w, err)
		return
	}
	defer stream.Close()
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Keep reverse proxies from buffering the stream. The browser should see
	// each token as soon as the provider produces it.
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}
	for {
		event, e := stream.Recv()
		if e == io.EOF {
			return
		}
		if e != nil {
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", jsonEscape(e.Error()))
			if flusher != nil {
				flusher.Flush()
			}
			return
		}
		if event != nil && event.Type == microagent.StreamEventDone {
			if err := g.agentStore.CompleteTurn(id, lease, stream.History()); err != nil {
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", jsonEscape("回复已生成，但会话保存失败，请重试"))
				if flusher != nil {
					flusher.Flush()
				}
				return
			}
		}
		b, _ := json.Marshal(event)
		eventName := "message"
		if event != nil && event.Type != "" {
			eventName = string(event.Type)
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, b); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
}

type tenantProvider struct {
	Code       string `json:"code"`
	ProviderID string `json:"provider_id"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Models     []any  `json:"models,omitempty"`
	Endpoint   string `json:"endpoint"`
	APIKey     string `json:"api_key"`
	Enabled    bool   `json:"enabled"`
	Default    bool   `json:"default"`
}

func resolveAgentProviderConfig(t *pb.Tenant, p payload) (provider, model, endpoint, apiKey string, err error) {
	if t == nil {
		return "", "", "", "", fault.NotFound
	}
	provider, model, endpoint, apiKey = p.str("provider"), p.str("model"), p.str("endpoint"), p.str("api_key", "apiKey")
	identity := p.str("code", "provider_id")
	index := int64(-1)
	if _, ok := p["index"]; ok {
		index = p.num("index")
	}
	if stored := providerBySelector(t.GetAgentProviders(), identityOrProvider(identity, provider), index); stored != nil {
		if provider == "" {
			provider = stored.Provider
		}
		if model == "" {
			model = stored.Model
		}
		if endpoint == "" {
			endpoint = stored.Endpoint
		}
		if strings.TrimSpace(apiKey) == "" || apiKey == "********" {
			apiKey = stored.APIKey
		}
	}
	if strings.TrimSpace(endpoint) == "" {
		return "", "", "", "", fault.Invalid("接口地址不能为空")
	}
	if strings.TrimSpace(apiKey) == "" || apiKey == "********" {
		return "", "", "", "", fault.Invalid("Provider API Key 未配置，请重新输入")
	}
	return provider, model, endpoint, apiKey, nil
}

func identityOrProvider(identity, provider string) string {
	if strings.TrimSpace(identity) != "" {
		return strings.TrimSpace(identity)
	}
	return strings.TrimSpace(provider)
}

func providerBySelector(raw, identity string, index int64) *tenantProvider {
	var list []tenantProvider
	if json.Unmarshal([]byte(raw), &list) != nil {
		return nil
	}
	if identity != "" {
		for i := range list {
			if providerIdentity(list[i]) == identity {
				return &list[i]
			}
		}
	}
	if index >= 0 && index < int64(len(list)) {
		return &list[index]
	}
	return nil
}

func providerIdentity(p tenantProvider) string {
	for _, value := range []string{p.Code, p.ProviderID, p.Provider, p.ID} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func defaultProvider(raw string) *tenantProvider {
	var list []tenantProvider
	if json.Unmarshal([]byte(raw), &list) != nil {
		return nil
	}
	for i := range list {
		if list[i].Enabled && list[i].Default {
			return &list[i]
		}
	}
	for i := range list {
		if list[i].Enabled {
			return &list[i]
		}
	}
	return nil
}

// maskTenantProviders keeps provider metadata available to the edit form while
// ensuring API keys never cross the HTTP boundary. Runtime execution uses the
// internal RPC value before this gateway response is masked.
func maskTenantProviders(raw string) string {
	var list []map[string]any
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	if json.Unmarshal([]byte(raw), &list) != nil {
		return "[]"
	}
	for _, item := range list {
		for key, value := range item {
			switch strings.ToLower(strings.ReplaceAll(key, "-", "_")) {
			case "api_key", "apikey":
				if s, ok := value.(string); ok && s != "" {
					item[key] = "********"
				}
			}
		}
	}
	b, err := json.Marshal(list)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func (g *Gateway) agentToolOptions(actor *pb.Context, auth *pb.AuthReply, request *http.Request) []agentcore.Tool {
	options := make([]agentcore.Tool, 0)
	checkCtx := context.Background()
	if request != nil {
		checkCtx = request.Context()
	}
	checkCtx, cancel := context.WithTimeout(checkCtx, 5*time.Second)
	defer cancel()
	appCode := g.agentAppCode(checkCtx, actor)
	// Tenant enumeration is a platform-management capability. Expose it as a
	// first-class tool so the model can distinguish “all tenants” from the
	// ordinary tenant-scoped CRUD routes. The service enforces PlatformAdmin
	// again, and credentials are deliberately excluded from the response.
	if auth.GetPlatformAdmin() && appCode == "system" {
		options = append(options, agentcore.Tool{Definition: ai.Tool{
			Name:        "tenant_list",
			Description: "查询平台中的全部租户。支持 keyword/name、page、page_size 参数；只返回租户基础信息，不包含 Provider 密钥。",
			Properties:  map[string]any{"payload": map[string]any{"type": "object", "additionalProperties": true, "description": "查询条件，可选 keyword、name、page、page_size"}},
		}, Handler: func(ctx context.Context, input map[string]any) (string, error) {
			if payload, ok := input["payload"].(map[string]any); ok && len(input) == 1 {
				input = payload
			}
			q := queryContext(actor, payload(input), pb.Kind_TENANTS)
			q.Id = 0
			q.TargetTenantId = 0
			q.IncludeAgentCredentials = false
			if q.PageSize == 0 {
				q.PageSize = 100
			}
			if q.PageSize > 100 {
				q.PageSize = 100
			}
			result, err := g.client.Query(rpcauth.WithCredential(ctx, g.config.GatewayKey), q)
			if err != nil {
				return "", err
			}
			b, _ := json.Marshal(result)
			return string(b), nil
		}})
	}
	options = append(options, g.agentProviderTools(actor, auth, appCode)...)
	for path, route := range g.routes {
		if path == "/healthz" || path == "/app/info/init" || !route.check {
			continue
		}
		if _, err := g.client.CheckAccess(rpcauth.WithCredential(checkCtx, g.config.GatewayKey), &pb.AccessRequest{Context: actor, AppCode: appCode, Method: route.method, Path: path}); err != nil {
			continue
		}
		// OpenAI-compatible tool schemas only accept [a-zA-Z0-9_-] in names.
		// Route paths may contain colons (for example system:tenant:main:view),
		// so keep the original path in the closure but expose a safe stable name.
		name := agentToolName(path)
		description := "调用 SaaS 接口 " + path + "。将请求参数放入 payload 对象。"
		if path == "/system/tenant/query" {
			description = "查询租户列表；在平台管理应用且具备平台管理员身份时返回全部租户，否则仅返回当前授权范围。将筛选和分页参数放入 payload 对象。"
		}
		options = append(options, agentcore.Tool{Definition: ai.Tool{Name: name, Description: description, Properties: map[string]any{
			"payload": map[string]any{"type": "object", "additionalProperties": true, "description": "接口请求参数"},
		}}, Handler: func(ctx context.Context, input map[string]any) (string, error) {
			if payload, ok := input["payload"].(map[string]any); ok && len(input) == 1 {
				input = payload
			}
			_, requiresConfirmation := agentRouteRisk(path, route.method)
			if requiresConfirmation && !boolValue(input["confirm"]) {
				return "", fault.New(409, "该操作需要用户确认")
			}
			callCtx := rpcauth.WithCredential(ctx, g.config.GatewayKey)
			if route.check {
				if _, err := g.client.CheckAccess(callCtx, &pb.AccessRequest{Context: actor, AppCode: appCode, Method: route.method, Path: path}); err != nil {
					return "", err
				}
			}
			result, err := route.call(callCtx, actor, payload(input))
			if err != nil {
				return "", err
			}
			b, _ := json.Marshal(result)
			return string(b), nil
		}})
	}
	return options
}

// agentProviderTools exposes provider administration only while the user is in
// the Basic application and has the provider resource (or an equivalent
// tenant-management grant). Provider credentials are used only inside the
// gateway and are never included in tool definitions or results.
func (g *Gateway) agentProviderTools(actor *pb.Context, auth *pb.AuthReply, appCode string) []agentcore.Tool {
	if actor == nil || auth == nil || appCode != "basic" || !canManageAgentProvider(auth) {
		return nil
	}
	queryTenant := func(ctx context.Context) (*pb.Tenant, error) {
		q, err := g.client.Query(rpcauth.WithCredential(ctx, g.config.GatewayKey), &pb.QueryRequest{Context: actor, Kind: pb.Kind_TENANTS, Id: actor.GetTenantId(), IncludeAgentCredentials: true})
		if err != nil {
			return nil, err
		}
		if q == nil || len(q.GetTenants()) == 0 {
			return nil, fault.NotFound
		}
		return q.GetTenants()[0], nil
	}
	selectProvider := func(t *pb.Tenant, input payload) payload {
		_, hasIndex := input["index"]
		if strings.TrimSpace(input.str("code", "provider_id", "provider")) == "" && !hasIndex {
			if p := defaultProvider(t.GetAgentProviders()); p != nil {
				input["code"] = providerIdentity(*p)
			} else if t.GetAgentEndpoint() != "" {
				// Preserve compatibility with tenants that still use the legacy
				// single-provider columns while they migrate to the JSON list.
				input["provider"], input["model"], input["endpoint"], input["api_key"] = t.GetAgentProvider(), t.GetAgentModel(), t.GetAgentEndpoint(), t.GetAgentApiKey()
			}
		}
		return input
	}
	toolPayload := func(input map[string]any) payload {
		p := payload(input)
		if nested, ok := input["payload"].(map[string]any); ok && len(input) == 1 {
			p = payload(nested)
		}
		return p
	}
	return []agentcore.Tool{
		{Definition: ai.Tool{Name: "provider_list", Description: "列出当前企业管理应用所属租户的 Agent Provider 配置、模型和默认状态，不返回 API 密钥。", Properties: map[string]any{
			"payload": map[string]any{"type": "object", "additionalProperties": false, "description": "无需参数"},
		}}, Handler: func(ctx context.Context, input map[string]any) (string, error) {
			t, err := queryTenant(ctx)
			if err != nil {
				return "", err
			}
			var providers []tenantProvider
			if strings.TrimSpace(t.GetAgentProviders()) != "" && json.Unmarshal([]byte(t.GetAgentProviders()), &providers) != nil {
				return "", fault.Invalid("Provider 配置格式错误")
			}
			if len(providers) == 0 && t.GetAgentEndpoint() != "" {
				providers = []tenantProvider{{Name: t.GetAgentProvider(), Provider: t.GetAgentProvider(), Model: t.GetAgentModel(), Endpoint: t.GetAgentEndpoint(), Enabled: t.GetAgentEnabled(), Default: t.GetAgentEnabled()}}
			}
			out := make([]map[string]any, 0, len(providers))
			for _, p := range providers {
				name := p.Name
				if name == "" {
					name = p.Code
				}
				if name == "" {
					name = p.Provider
				}
				out = append(out, map[string]any{"id": providerIdentity(p), "name": name, "provider": p.Provider, "model": p.Model, "endpoint": p.Endpoint, "models": p.Models, "enabled": p.Enabled, "default": p.Default})
			}
			b, _ := json.Marshal(map[string]any{"tenant_id": t.GetId(), "providers": out})
			return string(b), nil
		}},
		{Definition: ai.Tool{Name: "provider_models", Description: "同步当前租户已配置 Provider 的可用模型列表。可选 code、provider_id 或 index 选择 Provider；不接受外部接口地址或密钥。", Properties: map[string]any{
			"payload": map[string]any{"type": "object", "additionalProperties": true, "description": "可选 Provider 选择器 code、provider_id、index"},
		}}, Handler: func(ctx context.Context, input map[string]any) (string, error) {
			t, err := queryTenant(ctx)
			if err != nil {
				return "", err
			}
			p := selectProvider(t, toolPayload(input))
			_, _, endpoint, key, err := resolveAgentProviderConfig(t, p)
			if err != nil {
				return "", err
			}
			u, err := providerURL(endpoint, "/models")
			if err != nil {
				return "", fault.Invalid("Provider Endpoint 格式不正确")
			}
			models, err := fetchAgentProviderModels(ctx, u, key, g.config.AgentHTTPClient)
			if err != nil {
				return "", fault.New(502, err.Error())
			}
			b, _ := json.Marshal(map[string]any{"models": models})
			return string(b), nil
		}},
		{Definition: ai.Tool{Name: "provider_test", Description: "测试当前租户 Provider 的指定模型连通性。可选 code、provider_id、index 和 model；只测试已保存的 Provider。", Properties: map[string]any{
			"payload": map[string]any{"type": "object", "additionalProperties": true, "description": "可选 Provider 选择器和模型名称"},
		}}, Handler: func(ctx context.Context, input map[string]any) (string, error) {
			t, err := queryTenant(ctx)
			if err != nil {
				return "", err
			}
			p := selectProvider(t, toolPayload(input))
			_, model, endpoint, key, err := resolveAgentProviderConfig(t, p)
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(model) == "" {
				return "", fault.Invalid("模型不能为空")
			}
			u, err := providerURL(endpoint, "/chat/completions")
			if err != nil {
				return "", fault.Invalid("Provider Endpoint 格式不正确")
			}
			result, err := probeAgentModel(ctx, &http.Client{Timeout: 30 * time.Second}, u, model, key)
			if err != nil {
				return "", fault.New(502, err.Error())
			}
			b, _ := json.Marshal(result)
			return string(b), nil
		}},
	}
}

func agentRouteRisk(path, method string) (agentcore.RiskLevel, bool) {
	risk := agentcore.RiskRead
	confirm := false
	if method != http.MethodGet {
		risk, confirm = agentcore.RiskWrite, true
	}
	lower := strings.ToLower(path)
	for _, marker := range []string{"delete", "deauth", "resetpassword", "revoke"} {
		if strings.Contains(lower, marker) {
			return agentcore.RiskDelete, true
		}
	}
	for _, marker := range []string{"setstatus", "grant", "approve", "authroleresource"} {
		if strings.Contains(lower, marker) {
			return agentcore.RiskPermission, true
		}
	}
	return risk, confirm
}

func agentToolName(path string) string {
	raw := "http_" + strings.Trim(path, "/")
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}

func boolValue(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1"
	default:
		return false
	}
}

func mustContext(r *http.Request) *pb.Context { c, _ := RequestContext(r); return c }
func jsonEscape(s string) string              { b, _ := json.Marshal(s); return string(b) }

func pageContext(v any) (agentcore.PageContext, error) {
	var page agentcore.PageContext
	if v == nil {
		return page, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return page, err
	}
	err = json.Unmarshal(b, &page)
	return page, err
}

// A session may retain context only while the current grant set is unchanged.
func agentPermissionVersion(auth *pb.AuthReply) string {
	permissions := append([]string(nil), auth.GetPermissions()...)
	roles := append([]string(nil), auth.GetRoles()...)
	sort.Strings(permissions)
	sort.Strings(roles)
	raw, _ := json.Marshal(struct {
		Admin              bool
		Permissions, Roles []string
		Resources          []*pb.Resource
	}{auth.GetPlatformAdmin(), permissions, roles, auth.GetResources()})
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}
