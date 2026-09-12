package gateway

import (
	pb "kerthus/gen/go/saas/v1"
	"testing"
)

func TestAppDisplayFieldsAndEntitlementIdentityUseLegacyHTTPContract(t *testing.T) {
	m := &mockPlatform{}
	response := request(t, New(m, Config{}), "POST", "/system/app/save", `{"code":"demo","name":"Demo","icon":"logo.png","desc":"Introduction","remark":"Notes"}`)
	if response.num("code") != 0 || m.save.GetApp().Icon != "logo.png" || m.save.GetApp().Description != "Introduction" || m.save.GetApp().Remark != "Notes" {
		t.Fatal("app display fields lost at HTTP boundary")
	}
	app := appObject(&pb.App{Id: 99, TenantAppId: 321, Icon: "logo.png", Description: "Introduction", Remark: "Notes", ExpiresAt: 1900000000})
	if app.num("id") != 99 || app.num("tenant_app_id") != 321 || app.num("expiration_time") != 1900000000 || app.str("icon") != "logo.png" || app.str("desc") != "Introduction" || app.str("remark") != "Notes" {
		t.Fatal("app DTO confused catalog identity, entitlement or display fields")
	}
	m.query = nil
	request(t, New(m, Config{}), "GET", "/system/employee/queryTenantApps?page=2&pageSize=1", "")
	if m.query == nil || m.query.Page != 2 || m.query.PageSize != 1 || !m.query.TenantAppsOnly {
		t.Fatal("tenant app list discarded page or ownership context")
	}
	request(t, New(m, Config{}), "GET", "/system/position/getItems", "")
	if m.query == nil || !m.query.AvailableOnly {
		t.Fatal("position options include disabled entries")
	}
	request(t, New(m, Config{}), "GET", "/system/tenant/getItems", "")
	if m.query == nil || !m.query.AvailableOnly {
		t.Fatal("tenant options include unusable drafts")
	}
}

func TestDetailQueryRequiresExplicitPositiveIdentity(t *testing.T) {
	for _, group := range []string{"tenant", "org", "employee"} {
		for _, suffix := range []string{"", "?id=0", "?id=-1"} {
			m := &mockPlatform{}
			out := request(t, New(m, Config{}), "GET", "/system/"+group+"/info"+suffix, "")
			if out.num("code") != 400 || m.query != nil {
				t.Fatal("missing detail ID unexpectedly selected the first record", group, suffix)
			}
		}
	}
}
