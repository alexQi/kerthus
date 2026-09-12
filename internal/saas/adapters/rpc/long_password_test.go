package rpc

import (
	"strings"
	"testing"

	pb "kerthus/gen/go/saas/v1"
	c "kerthus/internal/saas/usecase/contracts"
)

func TestLongExistingPasswordUsecaseMapping(t *testing.T) {
	password := "  " + strings.Repeat("旧密码", 30) + "\x00suffix  "
	var login c.LoginRequest
	if err := mapValue(&pb.LoginRequest{Password: password}, &login); err != nil {
		t.Fatal(err)
	}
	var change c.PasswordRequest
	if err := mapValue(&pb.PasswordRequest{OldPassword: password, NewPassword: "new-password"}, &change); err != nil {
		t.Fatal(err)
	}
	if login.Password != password || change.OldPassword != password {
		t.Fatal("RPC usecase mapping changed existing credential bytes")
	}
}
