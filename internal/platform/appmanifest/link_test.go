package appmanifest

import "testing"

func TestManifestLinkResources(t *testing.T) {
	for _, mode := range []string{"inside", "outside"} {
		t.Run(mode, func(t *testing.T) {
			root := fixture(t, func(m, _ map[string]any) {
				link := map[string]any{"code": "notes:link", "name": "Link", "type": "menu", "open_with": mode, "path": "/notes/link", "component": "https://partner.example/embed"}
				if mode == "outside" {
					link["path"], link["component"] = "https://partner.example", ""
				}
				m["resources"] = append(m["resources"].([]map[string]any), link)
			})
			m, err := Load(root, "app.yaml")
			if err != nil {
				t.Fatal(err)
			}
			if m.Definition().Resources[1].OpenWith != mode {
				t.Fatal("manifest link mode lost")
			}
		})
	}
	for _, scenario := range []string{"unsafe", "external-parent", "external-home"} {
		root := fixture(t, func(m, _ map[string]any) {
			rs := m["resources"].([]map[string]any)
			rs = append(rs, map[string]any{"code": "notes:external", "name": "External", "type": "menu", "open_with": "outside", "path": "https://partner.example"})
			switch scenario {
			case "unsafe":
				rs[1]["path"] = "javascript:alert(1)"
			case "external-parent":
				rs[0]["parent_code"] = "notes:external"
			case "external-home":
				m["home"] = "https://partner.example"
			}
			m["resources"] = rs
		})
		if _, err := Load(root, "app.yaml"); err == nil {
			t.Fatalf("invalid link manifest accepted: %s", scenario)
		}
	}
}
