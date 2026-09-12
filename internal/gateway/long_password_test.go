package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go-micro.dev/v6/client"
	"google.golang.org/protobuf/proto"
	pb "kerthus/gen/go/saas/v1"
)

type longCredentialPlatform struct {
	mockPlatform
	loginPassword, oldPassword string
}

func (m *longCredentialPlatform) Login(ctx context.Context, in *pb.LoginRequest, opts ...client.CallOption) (*pb.LoginReply, error) {
	raw, err := proto.Marshal(in)
	if err != nil {
		return nil, err
	}
	var decoded pb.LoginRequest
	if err = proto.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	m.loginPassword = decoded.Password
	return m.mockPlatform.Login(ctx, in, opts...)
}
func (m *longCredentialPlatform) ChangePassword(_ context.Context, in *pb.PasswordRequest, _ ...client.CallOption) (*pb.Result, error) {
	raw, err := proto.Marshal(in)
	if err != nil {
		return nil, err
	}
	var decoded pb.PasswordRequest
	if err = proto.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	m.oldPassword = decoded.OldPassword
	return &pb.Result{Success: true}, nil
}

func TestLongExistingPasswordsSurviveHTTPAndProtoMapping(t *testing.T) {
	m := &longCredentialPlatform{}
	h := New(m, Config{GatewayKey: "test-service-key"})
	password := "  " + strings.Repeat("原密码", 30) + "\x00suffix  "
	for _, item := range []struct {
		path   string
		values map[string]any
	}{
		{"/system/user/login", map[string]any{"scene": "phone", "username": "13900000000", "password": password}},
		{"/system/user/modifyPassword", map[string]any{"old_password": password, "password": "replacement-password"}},
		{"/system/user/modifyInfo", map[string]any{"name": "Synthetic user", "email": "synthetic@example.invalid", "password": password}},
	} {
		body, err := json.Marshal(item.values)
		if err != nil {
			t.Fatal(err)
		}
		out := request(t, h, "POST", item.path, string(body))
		if out.num("code") != 0 {
			t.Fatalf("credential endpoint %s rejected request", item.path)
		}
	}
	if m.loginPassword != password || m.oldPassword != password || m.save.Password != password {
		t.Fatal("existing credential was truncated, normalized, or lost during transport mapping")
	}
}
