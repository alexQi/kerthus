package config

import "testing"

func TestAttachmentLimitConfiguration(t *testing.T) {
	for _, pair := range [][2]string{{"ENV_FILE", ""}, {"MYSQL_DSN", "fixture"}, {"REDIS_ADDRESS", "127.0.0.1:1"}, {"CONSUL_ADDRESS", "127.0.0.1:2"}, {"RPC_ADDRESS", "127.0.0.1:3"}, {"HTTP_ADDRESS", "127.0.0.1:4"}, {"RPC_GATEWAY_KEY", "local-test-key-000000000000000000"}, {"RPC_APP_KEYS", ""}, {"RPC_TLS_CA", ""}} {
		t.Setenv("KERTHUS_"+pair[0], pair[1])
	}
	for _, test := range []struct {
		raw   string
		want  int64
		valid bool
	}{{"", 50 << 20, true}, {"1048576", 1 << 20, true}, {"1073741824", 1 << 30, true}, {"0", 0, false}, {"-1", 0, false}, {"1048575", 0, false}, {"1073741825", 0, false}, {"50MB", 0, false}} {
		t.Setenv("KERTHUS_ATTACHMENT_MAX_BYTES", test.raw)
		cfg, err := Load()
		if (err == nil) != test.valid || (err == nil && cfg.AttachmentMaxBytes != test.want) {
			t.Fatalf("limit %q: value=%d err=%v", test.raw, cfg.AttachmentMaxBytes, err)
		}
	}
}
