package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizePageContextReplacesIdentityAndRedactsSecrets(t *testing.T) {
	out, err := NormalizePageContext(PageContext{
		Version:  "old",
		TenantID: 999,
		Filters:  map[string]any{"name": "张三", "password": "do-not-send", "api_key": "api-secret", "accessKey": "access-secret", "nested": map[string]any{"token": "x", "ok": "yes"}},
	}, TrustedContext{UserID: 7, TenantID: 12, AppID: 3})
	if err != nil {
		t.Fatal(err)
	}
	if out.Page.TenantID != 12 || out.Page.AppID != 3 || out.Page.Version != PageContextVersion {
		t.Fatalf("unexpected trusted context: %+v", out.Page)
	}
	b, _ := json.Marshal(out)
	if strings.Contains(string(b), "do-not-send") || strings.Contains(string(b), "api-secret") || strings.Contains(string(b), "access-secret") || strings.Contains(string(b), "token") {
		t.Fatalf("secret leaked: %s", b)
	}
}

func TestNormalizePageContextLimitsPayload(t *testing.T) {
	data := map[string]any{}
	for i := 0; i < 100; i++ {
		data[string(rune('a'+i%26))+strings.Repeat("x", i)] = strings.Repeat("y", 1024)
	}
	_, err := NormalizePageContext(PageContext{FormData: data}, TrustedContext{})
	if err == nil {
		t.Fatal("expected oversized context to be rejected")
	}
}
