package gateway

import (
	"context"
	"testing"

	"go-micro.dev/v6/client"
	pb "kerthus/gen/go/saas/v1"
)

type legacyReviewPlatform struct{ mockPlatform }

func (m *legacyReviewPlatform) Query(_ context.Context, q *pb.QueryRequest, _ ...client.CallOption) (*pb.QueryReply, error) {
	m.query = q
	return &pb.QueryReply{}, nil
}

func TestLegacyCatalogFiltersAndDisplaySortingReachRPC(t *testing.T) {
	m := &legacyReviewPlatform{}
	h := New(m, Config{})
	for _, tc := range []struct {
		path, sort string
		desc       bool
	}{
		{"/system/org/query", "sort", false},
		{"/system/app/getAppResources?app_id=5", "sort", false},
		{"/system/app/getAppResources?app_id=5&field=name&order=descend", "name", true},
	} {
		out := request(t, h, "GET", tc.path, "")
		if out.num("code") != 0 || m.query.Sort != tc.sort || m.query.Desc != tc.desc {
			t.Fatalf("display order lost: %s", tc.path)
		}
	}
	request(t, h, "GET", "/system/app/query?name=Foundation&code=basic&page=2&pageSize=1", "")
	if m.query.Search != "Foundation" || m.query.Code != "basic" || m.query.Page != 2 {
		t.Fatal("application code/name filters or pagination lost")
	}
	request(t, h, "GET", "/system/app/queryTeantAuthorizes?tenant_name=Acme&app_name=Notes&page=2&pageSize=1", "")
	if m.query.TenantName != "Acme" || m.query.AppName != "Notes" || m.query.Page != 2 || m.query.PageSize != 1 {
		t.Fatal("entitlement filters/pagination lost")
	}
}

func TestLegacyResourceRedirectAndRemarkRoundTrip(t *testing.T) {
	m := &mockPlatform{}
	out := request(t, New(m, Config{}), "POST", "/system/app/saveResource", `{"app_id":5,"name":"Folder","code":"basic:folder","type":"menu","redirect":"/basic/home","remark":"Menu note"}`)
	if out.num("code") != 0 || m.save.GetResource().Redirect != "/basic/home" || m.save.GetResource().Remark != "Menu note" {
		t.Fatal("legacy resource fields discarded on save")
	}
	for _, nav := range []bool{false, true} {
		r := resourceObject(m.save.GetResource(), nav)
		if r.str("redirect") != "/basic/home" || r.str("remark") != "Menu note" {
			t.Fatal("resource detail/navigation lost redirect or remark")
		}
	}
	if resourceObject(&pb.Resource{OpenWith: "outside"}, false).str("open_with") != "outside" {
		t.Fatal("unsupported legacy route mode disguised as a component")
	}
}

func TestResourceDetailRequiresAnExplicitIdentity(t *testing.T) {
	for _, suffix := range []string{"", "?id=0", "?id=-1"} {
		m := &legacyReviewPlatform{}
		out := request(t, New(m, Config{}), "GET", "/system/app/getResource"+suffix, "")
		if out.num("code") != 400 || m.query != nil {
			t.Fatal("missing resource identity must not return the first record")
		}
	}
}
