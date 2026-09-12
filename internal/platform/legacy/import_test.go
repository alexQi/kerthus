package legacy

import (
	"kerthus/internal/platform/appmanifest"
	"os"
	"path/filepath"
	"testing"
)

func TestBindingCollapseRequiresEqualEffectivePermission(t *testing.T) {
	roles := map[int64]Row{1: {"id": 1}, 17: {"id": 17}}
	bindings := map[string][]int64{"2 POST /system/employee/save": {92, 93}}
	allowed := map[string]int64{"1:92": 0, "1:93": 0}
	if e := validateBindings(bindings, roles, allowed); e != nil {
		t.Fatal(e)
	}
	allowed["17:92"] = 0
	if e := validateBindings(bindings, roles, allowed); e == nil {
		t.Fatal("partial permission was expanded")
	}
	allowed["17:93"] = 5
	if e := validateBindings(bindings, roles, allowed); e == nil {
		t.Fatal("different data scopes collapsed")
	}
}
func TestMissingPositionOrgRequiresUnanimousEvidence(t *testing.T) {
	p := Row{"id": 32, "tenant_id": 1, "org_id": 3}
	orgs := map[int64]Row{46: {"id": 46, "tenant_id": 1}, 47: {"id": 47, "tenant_id": 1}}
	s := map[string][]Row{"d_tenant_employee_position": {{"position_id": 32, "tenant_id": 1, "user_id": 9}, {"position_id": 32, "tenant_id": 1, "user_id": 10}}, "d_tenant_employee_org": {{"tenant_id": 1, "user_id": 9, "org_id": 46}, {"tenant_id": 1, "user_id": 10, "org_id": 46}}}
	got, e := positionOrg(p, s, orgs)
	if e != nil || got != 46 {
		t.Fatalf("%d %v", got, e)
	}
	s["d_tenant_employee_org"][1]["org_id"] = 47
	if _, e := positionOrg(p, s, orgs); e == nil {
		t.Fatal("conflicting organization repaired by guessing")
	}
}
func TestCatalogKeepsResourceIdentityAndDisablesUnsupportedBusiness(t *testing.T) {
	root := t.TempDir()
	view := filepath.Join(root, "web/admin/src/views/common/dashboard")
	os.MkdirAll(view, 0755)
	os.WriteFile(filepath.Join(view, "index.vue"), []byte("<template/>"), 0644)
	apps := []Row{{"id": 2, "code": "basic", "name": "Basic", "status": 1}, {"id": 41, "code": "noctua", "name": "Old app", "status": 1}}
	resources := []Row{{"id": 4, "app_id": 2, "code": "basic:dashboard", "name": "Home", "type": "menu", "path": "/basic/dashboard", "component": "/basic/dashboard/index", "status": 1, "meta": "{}"}, {"id": 92, "app_id": 2, "code": "basic:user:main:create", "type": "action", "name": "Create", "status": 1, "meta": "{}"}, {"id": 93, "app_id": 2, "code": "basic:user:main:edit", "type": "action", "name": "Edit", "status": 1, "meta": "{}"}}
	apis := []Row{{"resource_id": 92, "method": "POST", "uri": "/system/employee/save"}, {"resource_id": 93, "method": "POST", "uri": "/system/employee/save"}}
	ops := []appmanifest.Operation{{ID: "employee.save", AppCode: "basic", Method: "POST", Path: "/system/employee/save", Action: "membership.write"}}
	m, e := Catalog(apps, resources, apis, ops, root)
	if e != nil {
		t.Fatal(e)
	}
	if m.Resources[0].ID != 4 || m.Resources[0].Code != "basic:dashboard" || m.Resources[0].Component != "common/dashboard/index" || m.Apps[1].Status != 0 {
		t.Fatal("identity or application boundary changed")
	}
	if len(m.Operations) != 1 || m.Operations[0].ResourceID != 92 || len(m.BindingCandidates["2 POST /system/employee/save"]) != 2 {
		t.Fatal("OR-binding evidence lost")
	}
}
func TestTargetDatabaseNames(t *testing.T) {
	for _, name := range []string{"beehive_saas", "kerthus_`;DROP DATABASE foo", "mysql", ""} {
		if databaseName(name) {
			t.Fatal("unsafe target accepted")
		}
	}
	if !databaseName("kerthus_saas") {
		t.Fatal("valid isolated database rejected")
	}
}

func TestCatalogPreservesLegacyApplicationDisplayMetadata(t *testing.T) {
	apps := []Row{{"id": 2, "code": "basic", "name": "Basic", "status": 1, "icon": "old/logo.png", "desc": "旧应用简介", "remark": "旧应用备注"}, {"id": 41, "code": "noctua", "name": "Business", "status": 1, "icon": "old/business.png", "desc": "业务说明", "remark": "保留目录"}}
	result, err := Catalog(apps, nil, nil, nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i, app := range result.Apps {
		if app.Icon != String(apps[i], "icon") || app.Description != String(apps[i], "desc") || app.Remark != String(apps[i], "remark") {
			t.Fatal("import dropped application display metadata")
		}
	}
	if result.Apps[1].Status != 0 {
		t.Fatal("metadata preservation activated a business application")
	}
}
