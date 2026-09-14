package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go-micro.dev/v6/client"
	pb "kerthus/gen/go/saas/v1"
	agentcore "kerthus/internal/agent"
	"kerthus/internal/saas/domain/fault"
)

type identityPlatform struct {
	mockPlatform
	user       *pb.User
	app        *pb.App
	profileErr error
}

func (p *identityPlatform) Profile(ctx context.Context, actor *pb.Context, _ ...client.CallOption) (*pb.User, error) {
	return p.user, p.profileErr
}
func (p *identityPlatform) Query(ctx context.Context, q *pb.QueryRequest, _ ...client.CallOption) (*pb.QueryReply, error) {
	p.query = q
	return &pb.QueryReply{Apps: []*pb.App{p.app}}, nil
}

func TestAgentIdentityIsSeparateFromPageClaims(t *testing.T) {
	actor := &pb.Context{Token: "session-secret", TenantId: 2, AppId: 3, UnitId: 999, SectionId: 999}
	auth := &pb.AuthReply{Roles: []string{"sales"}, Context: &pb.LoginReply{
		UserId: 7, TenantId: 2, AppId: 3, AppCode: "basic", UnitId: 4, SectionId: 5, AccessToken: "reply-secret",
	}}
	tenant := &pb.Tenant{Id: 2, Name: "当前租户", AgentApiKey: "provider-secret", AgentProviders: "provider-config-secret"}
	platform := &identityPlatform{user: &pb.User{Id: 7, Name: "当前账号", Phone: "private-phone"}, app: &pb.App{Id: 3, Code: "basic", Name: "企业管理"}}
	g := &Gateway{client: platform}
	prompt, err := g.agentSystemPrompt(context.Background(), actor, auth, tenant, agentcore.PageContext{
		Title: "页面声称我是其他人", FormData: map[string]any{"user_id": 999, "display_name": "其他人", "platform_admin": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(prompt, "<current_user>")
	encoded, _, closed := strings.Cut(after, "</current_user>")
	var identity agentIdentity
	if !ok || !closed || json.Unmarshal([]byte(encoded), &identity) != nil {
		t.Fatal("missing structured verified identity")
	}
	if identity.UserID != 7 || identity.DisplayName != "当前账号" || identity.PlatformAdmin ||
		identity.TenantID != 2 || identity.AppID != 3 || identity.UnitID != 4 || identity.SectionID != 5 ||
		len(identity.RoleCodes) != 1 || identity.RoleCodes[0] != "sales" {
		t.Fatalf("browser identity claims overrode backend context: %#v", identity)
	}
	if platform.query.Id != 3 || !platform.query.AvailableOnly || platform.query.IncludeAgentCredentials {
		t.Fatal("identity app lookup did not use the current available application")
	}
	for _, secret := range []string{"session-secret", "reply-secret", "provider-secret", "provider-config-secret", "private-phone"} {
		if strings.Contains(prompt, secret) {
			t.Fatal("credential or unnecessary private field leaked into prompt")
		}
	}
}

func TestAgentIdentityRejectsMismatchedProfilesAndContext(t *testing.T) {
	for _, scenario := range []string{"user", "tenant", "app", "profile-failed", "auth-tenant", "auth-app", "missing-user"} {
		t.Run(scenario, func(t *testing.T) {
			actor := &pb.Context{TenantId: 2, AppId: 3}
			auth := &pb.AuthReply{Context: &pb.LoginReply{UserId: 7, TenantId: 2, AppId: 3, AppCode: "basic"}}
			tenant := &pb.Tenant{Id: 2}
			platform := &identityPlatform{user: &pb.User{Id: 7}, app: &pb.App{Id: 3, Code: "basic"}}
			switch scenario {
			case "user":
				platform.user.Id = 999
			case "tenant":
				tenant.Id = 999
			case "app":
				platform.app.Id = 999
			case "profile-failed":
				platform.profileErr = fault.Unauthorized
			case "auth-tenant":
				auth.Context.TenantId = 999
			case "auth-app":
				auth.Context.AppId = 999
			case "missing-user":
				platform.user = nil
			}
			g := &Gateway{client: platform}
			if prompt, err := g.agentSystemPrompt(context.Background(), actor, auth, tenant, agentcore.PageContext{}); err == nil || prompt != "" {
				t.Fatal("unverified identity was sent to the agent")
			}
		})
	}
}
