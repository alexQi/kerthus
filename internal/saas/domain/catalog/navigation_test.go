package catalog

import "testing"

func TestBrowserDestinations(t *testing.T) {
	for _, value := range []string{"https://example.com/path?q=中文#chapter", "HTTPS://example.com/path", "http://127.0.0.1:15175/page", "https://[::1]/"} {
		if !SafeWebURL(value) {
			t.Errorf("valid browser URL rejected: %q", value)
		}
	}
	for _, value := range []string{"", "//evil.test/path", "javascript:alert(1)", "data:text/html,hello", "file:///etc/passwd", "https://user:pass@example.com", "https:example.com", "https://", "https://evil.test\\@good.test", "https://example.com/\npath", " https://example.com", "https://example.com:abc/"} {
		if SafeWebURL(value) {
			t.Errorf("unsafe browser URL accepted: %q", value)
		}
	}
}

func TestComponentOwnership(t *testing.T) {
	for _, value := range []string{"apps/notes/index", "/apps/notes/index", "LAYOUT", ""} {
		if !ComponentReference("notes", value) {
			t.Errorf("valid component rejected: %q", value)
		}
	}
	for _, value := range []string{"apps/other/index", "../system/users", "https://example.com", "apps/notes/index?x=1", "apps/notes/a\\b", "//apps/notes/index", "apps/notes/%2e%2e/system"} {
		if ComponentReference("notes", value) {
			t.Errorf("unsafe component accepted: %q", value)
		}
	}
}
