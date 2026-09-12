package upload

import (
	"net/http/httptest"
	"testing"
)

func TestLegacyImagesUseOnlyConfiguredOrigin(t *testing.T) {
	h := FilesWithLegacy(nil, "https://legacy.example/assets/")
	for _, test := range []struct {
		path     string
		status   int
		location string
	}{{"/files/tenant/logo.png", 302, "https://legacy.example/assets/tenant/logo.png"}, {"/files/../secret", 404, ""}, {"/files/https://attacker.example/image", 404, ""}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", test.path, nil)
		h.ServeHTTP(w, r)
		if w.Code != test.status || w.Header().Get("Location") != test.location {
			t.Fatalf("%s: %d %s", test.path, w.Code, w.Header().Get("Location"))
		}
	}
}
