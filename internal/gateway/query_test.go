package gateway

import (
	pb "kerthus/gen/go/saas/v1"
	"testing"
)

func TestIdentitySearchAndOrganizationFiltersReachRPC(t *testing.T) {
	for _, path := range []string{"/system/employee/query", "/system/user/query"} {
		m := &mockPlatform{}
		out := request(t, New(m, Config{}), "GET", path+"?name=Alice&phone=0003&keyword=example.test&org_id=31&with_child=1&children_ids[]=999&tenant_id=12&page=2&pageSize=1", "")
		q := m.query
		if out.num("code") != 0 || q == nil || q.Name != "Alice" || q.Phone != "0003" || q.Search != "example.test" || q.OrgId != 31 || !q.IncludeChildren || q.TargetTenantId != 12 || q.Page != 2 || q.PageSize != 1 {
			t.Fatal("legacy search filters were dropped or conflated")
		}
	}
	q := queryContext(&pb.Context{}, payload{"keyword": "", "name": " Alice ", "phone": " 0003 "}, pb.Kind_MEMBERS)
	if q.Name != "Alice" || q.Phone != "0003" || q.Search != "" {
		t.Fatal("blank keyword masked field-specific identity filters")
	}
	q = queryContext(&pb.Context{}, payload{"keyword": "", "name": " Department "}, pb.Kind_ORGS)
	if q.Search != "Department" {
		t.Fatal("blank keyword masked organization name search")
	}
}
