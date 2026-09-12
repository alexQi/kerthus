package gateway

import (
	"context"
	"testing"

	"go-micro.dev/v6/client"
	microerrors "go-micro.dev/v6/errors"
	pb "kerthus/gen/go/saas/v1"
)

type linkPlatform struct {
	mockPlatform
	available  bool
	authCalled bool
}

func (m *linkPlatform) Query(_ context.Context, q *pb.QueryRequest, _ ...client.CallOption) (*pb.QueryReply, error) {
	m.query = q
	if !m.available {
		return nil, microerrors.NotFound("saas", "not available")
	}
	out := &pb.QueryReply{}
	if m.available {
		out.Apps = []*pb.App{{Id: 7, Type: "third", Url: "https://partner.example", IsPublic: true}}
		out.Total = 1
	}
	return out, nil
}
func (m *linkPlatform) Auth(_ context.Context, _ *pb.Context, _ ...client.CallOption) (*pb.AuthReply, error) {
	m.authCalled = true
	return &pb.AuthReply{}, nil
}
func TestThirdPartyAvailabilityUsesEntitlementWithoutChangingContext(t *testing.T) {
	m := &linkPlatform{available: true}
	h := New(m, Config{})
	out := request(t, h, "GET", "/system/employee/hasApp?app_id=7", "")
	if out.num("code") != 0 || out["data"] != true || m.authCalled || !m.query.AvailableOnly || m.query.Id != 7 {
		t.Fatal("third-party availability bypassed entitlement query or selected a backend context")
	}
	m.available = false
	out = request(t, h, "GET", "/system/employee/hasApp?app_id=7", "")
	if out["data"] != false {
		t.Fatal("revoked third-party link is available")
	}
}
func TestOutsideResourceOnlyAppearsInMenu(t *testing.T) {
	resources := []*pb.Resource{{Id: 1, Type: "menu", Name: "Outside", OpenWith: "outside", Path: "https://partner.example"}, {Id: 2, Type: "view", Name: "Inside", OpenWith: "inside", Path: "/basic/embed", Component: "https://partner.example/embed"}}
	if len(resourceTree(resources, true)) != 1 || len(menuTree(resources)) != 2 || len(resourceTree(resources, false)) != 2 {
		t.Fatal("external link registered as route or missing from menus/admin")
	}
	app := appObject(&pb.App{Type: "third", Url: "https://partner.example", IsPublic: true})
	if app.str("type") != "third" || app.str("url") == "" || !app.boolean("is_public") {
		t.Fatal("application launch metadata lost")
	}
}
