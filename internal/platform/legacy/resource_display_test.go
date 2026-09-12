package legacy

import "testing"

func TestCatalogPreservesResourceRedirectAndRemark(t *testing.T) {
	apps := []Row{{"id": 2, "code": "basic", "name": "Basic", "status": 1}}
	resources := []Row{{"id": 9, "app_id": 2, "code": "basic:folder", "name": "Folder", "type": "menu", "path": "/basic/folder", "component": "LAYOUT", "redirect": "/basic/dashboard", "remark": "Original note", "status": 1, "meta": "{}", "sort": 7}}
	out, err := Catalog(apps, resources, nil, nil, t.TempDir())
	if err != nil || len(out.Resources) != 1 {
		t.Fatal("catalog failed", err)
	}
	r := out.Resources[0]
	if r.ID != 9 || r.Redirect != "/basic/dashboard" || r.Remark != "Original note" || r.Sort != 7 {
		t.Fatal("resource display fields lost during import")
	}
	resources[0]["open_with"] = "outside"
	out, err = Catalog(apps, resources, nil, nil, t.TempDir())
	if err != nil || out.Resources[0].OpenWith != "outside" || out.Resources[0].Status != 0 {
		t.Fatal("legacy unsupported mode must remain identifiable and disabled", err)
	}
}
