package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-micro.dev/v6/client"
	pb "kerthus/gen/go/saas/v1"
	agentcore "kerthus/internal/agent"
	"kerthus/internal/saas/domain/fault"
)

type conversationPlatform struct {
	mockPlatform
	name string
}

func (p *conversationPlatform) Auth(ctx context.Context, c *pb.Context, _ ...client.CallOption) (*pb.AuthReply, error) {
	if c.GetToken() != "test-token" {
		return nil, fault.Unauthorized
	}
	return &pb.AuthReply{PlatformAdmin: true, Roles: []string{"admin"}, Context: &pb.LoginReply{
		UserId: 1, TenantId: c.GetTenantId(), AppId: c.GetAppId(), AppCode: "basic", UnitId: 4, SectionId: 5,
	}}, nil
}
func (p *conversationPlatform) Profile(ctx context.Context, c *pb.Context, _ ...client.CallOption) (*pb.User, error) {
	return &pb.User{Id: 1, Name: p.name, Phone: "private-phone", Email: "private-email"}, nil
}
func (p *conversationPlatform) Query(ctx context.Context, q *pb.QueryRequest, _ ...client.CallOption) (*pb.QueryReply, error) {
	if q.Kind == pb.Kind_APPS {
		return &pb.QueryReply{Apps: []*pb.App{{Id: 3, Code: "basic", Name: "企业管理"}}}, nil
	}
	key := "********"
	if q.IncludeAgentCredentials {
		key = "stored-test-key"
	}
	return &pb.QueryReply{Tenants: []*pb.Tenant{{Id: 2, Name: "测试租户", AgentEnabled: true, AgentProvider: "openai", AgentModel: "test", AgentEndpoint: "https://provider.example/v1", AgentApiKey: key}}}, nil
}

type rewriteAgentTransport struct{ target string }

func (t rewriteAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	u := *req.URL
	clone.URL = &u
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(t.target, "http://")
	return http.DefaultTransport.RoundTrip(clone)
}
func conversationRequest(method, url, body string) *http.Request {
	r, _ := http.NewRequest(method, url, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("access-token", "test-token")
	r.Header.Set("tenant-id", "2")
	r.Header.Set("app-id", "3")
	return r
}
func TestAgentHTTPStreamsBeforeCompletionAndRestoresHistory(t *testing.T) {
	release := make(chan struct{})
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream   bool                `json:"stream"`
			Messages []agentcore.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if !body.Stream || r.Header.Get("Authorization") != "Bearer stored-test-key" {
			t.Error("stream request did not use stored credentials")
		}
		if len(body.Messages) == 0 || body.Messages[0].Role != "system" {
			t.Error("missing trusted system context")
			return
		}
		prompt := body.Messages[0].Content
		for _, required := range []string{`"user_id":1`, `"tenant_id":2`, `"tenant_name":"测试租户"`, `"app_id":3`, `"app_name":"企业管理"`, `"role_codes":["admin"]`, `"unit_id":4`, `"section_id":5`} {
			if !strings.Contains(prompt, required) {
				t.Errorf("upstream did not receive verified identity: missing %s", required)
			}
		}
		for _, secret := range []string{"stored-test-key", "test-token", "private-phone", "private-email"} {
			if strings.Contains(prompt, secret) {
				t.Error("unnecessary private fields entered system context")
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if requests.Add(1) == 1 {
			if !strings.Contains(prompt, `"display_name":"初始账号"`) {
				t.Error("first turn did not load profile")
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"记住蓝莓\"}}]}\n\n")
			w.(http.Flusher).Flush()
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		} else {
			if !strings.Contains(prompt, `"display_name":"更新账号"`) || strings.Contains(prompt, "初始账号") {
				t.Error("continued conversation reused stale profile context")
			}
			if len(body.Messages) != 4 || body.Messages[1].Content != "记住蓝莓" || body.Messages[2].Content != "记住蓝莓" || body.Messages[3].Content != "刚才的词是什么" {
				t.Errorf("history lost between HTTP requests: %#v", body.Messages)
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"蓝莓\"}}]}\n\n")
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	store := agentcore.NewSessionStore()
	conf := Config{AgentStore: store, AgentHTTPClient: &http.Client{Transport: rewriteAgentTransport{upstream.URL}}}
	server := httptest.NewServer(New(&conversationPlatform{name: "初始账号"}, conf))
	defer server.Close()
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Do(conversationRequest("POST", server.URL+"/api/agent/sessions", `{"context":{"route":"/basic/dashboard"}}`))
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Data agentcore.Session `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.Data.ID == "" {
		t.Fatal("session not created")
	}
	endpoint := "/api/agent/sessions/" + created.Data.ID
	resp, err = c.Do(conversationRequest("POST", server.URL+endpoint+"/messages", `{"message":"记住蓝莓"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected SSE: %s", b)
	}
	scanner := bufio.NewScanner(resp.Body)
	token := make(chan bool, 1)
	go func() {
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "记住蓝莓") {
				token <- true
				return
			}
		}
		token <- false
	}()
	select {
	case ok := <-token:
		if !ok {
			t.Fatal("missing real-time token")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("gateway buffered answer until upstream finished")
	}
	// A duplicate concurrent request must not start another model run.
	busy, err := c.Do(conversationRequest("POST", server.URL+endpoint+"/messages", `{"message":"duplicate"}`))
	if err != nil {
		t.Fatal(err)
	}
	var busyBody payload
	json.NewDecoder(busy.Body).Decode(&busyBody)
	busy.Body.Close()
	if busyBody.num("code") != 409 {
		t.Fatalf("concurrent turn accepted: %#v", busyBody)
	}
	close(release)
	done := false
	for scanner.Scan() {
		if scanner.Text() == "event: done" {
			done = true
		}
	}
	if !done {
		t.Fatal("turn was not committed with done")
	}
	resp.Body.Close()
	// New gateway instance, same session backend: retrieval and next turn work.
	reopened := httptest.NewServer(New(&conversationPlatform{name: "更新账号"}, conf))
	defer reopened.Close()
	restored, err := c.Do(conversationRequest("GET", reopened.URL+endpoint, ""))
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Data agentcore.Session `json:"data"`
	}
	json.NewDecoder(restored.Body).Decode(&record)
	restored.Body.Close()
	if len(record.Data.History) != 2 {
		t.Fatalf("history not exposed for refresh: %#v", record.Data.History)
	}
	foreign := conversationRequest("GET", reopened.URL+endpoint, "")
	foreign.Header.Set("tenant-id", "999")
	denied, err := c.Do(foreign)
	if err != nil {
		t.Fatal(err)
	}
	var deniedBody payload
	json.NewDecoder(denied.Body).Decode(&deniedBody)
	denied.Body.Close()
	if deniedBody.num("code") != 403 {
		t.Fatal("cross-tenant history accessible")
	}
	next, err := c.Do(conversationRequest("POST", reopened.URL+endpoint+"/messages", `{"message":"刚才的词是什么"}`))
	if err != nil {
		t.Fatal(err)
	}
	result, _ := io.ReadAll(next.Body)
	next.Body.Close()
	if !strings.Contains(string(result), "蓝莓") || !strings.Contains(string(result), "event: done") {
		t.Fatalf("continuation failed: %s", result)
	}
	if requests.Load() != 2 {
		t.Fatal("unexpected duplicate upstream requests")
	}
}
