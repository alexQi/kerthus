package appmanifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, mutate func(map[string]any, map[string]any)) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "web/admin/src/views/apps/notes")
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(dir, "index.vue"), []byte("<template>notes</template>"), 0644)
	m := map[string]any{"schema_version": 1, "app_code": "notes", "name": "Notes", "release_version": "1.0.0", "platform_api_version": "v1", "resource_version": 1, "http_contract": "contract.yaml", "service_key": "kerthus.notes", "route_prefix": "/api/apps/notes/v1", "frontend_entry": "bundled", "home": "/notes/home", "resources": []map[string]any{{"code": "notes:home", "name": "Notes", "type": "view", "path": "/notes/home", "component": "apps/notes/index"}}}
	spec := map[string]any{"openapi": "3.0.3", "paths": map[string]any{"/api/apps/notes/v1/notes/{id}": map[string]any{"get": map[string]any{"operationId": "notes.read", "x-app-code": "notes", "x-resource-code": "notes:home", "x-action": "notes.read"}}}}
	if mutate != nil {
		mutate(m, spec)
	}
	for path, v := range map[string]any{"app.yaml": m, "contract.yaml": spec} {
		b, _ := json.Marshal(v)
		if e := os.WriteFile(filepath.Join(root, path), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	return root
}
func TestManifestVersionOwnershipAndComponents(t *testing.T) {
	root := fixture(t, nil)
	m, e := Load(root, "app.yaml")
	if e != nil || len(m.Operations) != 1 {
		t.Fatalf("valid manifest: %v", e)
	}
	a, _ := m.Fingerprint()
	n, e := Load(root, "app.yaml")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := n.Fingerprint()
	if a != b {
		t.Fatal("unstable fingerprint")
	}
	cases := map[string]func(map[string]any, map[string]any){"incompatible": func(m, s map[string]any) { m["platform_api_version"] = "v2" }, "prefix": func(m, s map[string]any) { m["route_prefix"] = "/system" }, "component_missing": func(m, s map[string]any) { m["resources"].([]map[string]any)[0]["component"] = "missing" }, "namespace": func(m, s map[string]any) { m["resources"].([]map[string]any)[0]["code"] = "system:admin" }, "traversal": func(m, s map[string]any) { m["http_contract"] = "../outside.yaml" }, "foreign_operation": func(m, s map[string]any) {
		s["paths"].(map[string]any)["/api/apps/notes/v1/notes/{id}"].(map[string]any)["get"].(map[string]any)["x-app-code"] = "other"
	}}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			if _, e := Load(fixture(t, fn), "app.yaml"); e == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}

func TestAmbiguousStaticAndParameterRoutesRejected(t *testing.T) {
	root := fixture(t, func(m, s map[string]any) {
		s["paths"].(map[string]any)["/api/apps/notes/v1/notes/search"] = map[string]any{"get": map[string]any{"operationId": "notes.search", "x-app-code": "notes", "x-resource-code": "notes:home", "x-action": "notes.search"}}
	})
	if _, e := Load(root, "app.yaml"); e == nil {
		t.Fatal("ambiguous routes accepted")
	}
}
