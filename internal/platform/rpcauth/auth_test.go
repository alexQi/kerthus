package rpcauth

import (
	"context"
	"go-micro.dev/v6/metadata"
	"kerthus/internal/saas/domain/fault"
	"strings"
	"testing"
)

func TestServicePrincipals(t *testing.T) {
	gateway := strings.Repeat("g", 32)
	app := strings.Repeat("a", 32)
	a, e := New(Config{GatewayKey: gateway, AppKeys: map[string]string{"notes": app}})
	if e != nil {
		t.Fatal(e)
	}
	tests := []struct {
		name, key, method, code string
		want                    int32
	}{{"missing", "", "Login", "", 401}, {"unknown", strings.Repeat("x", 32), "Health", "", 401}, {"gateway", gateway, "Save", "", 0}, {"app own", app, "CheckAccess", "notes", 0}, {"app other", app, "CheckAccess", "crm", 403}, {"app health", app, "Health", "", 0}, {"app login", app, "Login", "", 403}, {"app mutation", app, "Save", "", 403}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithCredential(context.Background(), tt.key)
			ctx = metadata.Set(ctx, "actor-id", "1")
			ctx = metadata.Set(ctx, "platform-admin", "true")
			e := a.Authorize(ctx, tt.method, tt.code)
			var code int32
			if e != nil {
				code, _ = fault.Code(e)
			}
			if code != tt.want {
				t.Fatalf("got %d want %d", code, tt.want)
			}
		})
	}
}
func TestCredentialConfiguration(t *testing.T) {
	key := strings.Repeat("x", 32)
	for _, c := range []Config{{GatewayKey: "short"}, {GatewayKey: key, AppKeys: map[string]string{"notes": key}}, {GatewayKey: key, AppKeys: map[string]string{"notes": "short"}}} {
		if _, e := New(c); e == nil {
			t.Fatal("accepted unsafe credential configuration")
		}
	}
}
