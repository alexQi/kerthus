package gateway

import (
	"context"
	"reflect"
	"testing"

	"go-micro.dev/v6/client"
	pb "kerthus/gen/go/saas/v1"
)

type authorizationCatalogPlatform struct{ mockPlatform }

func (m *authorizationCatalogPlatform) Query(ctx context.Context, q *pb.QueryRequest, opts ...client.CallOption) (*pb.QueryReply, error) {
	if q.Kind == pb.Kind_APPS {
		return &pb.QueryReply{Total: 2, Apps: []*pb.App{
			{Id: 1, Code: "system", Name: "平台管理"},
			{Id: 2, Code: "basic", Name: "企业管理"},
		}}, nil
	}
	resources := []*pb.Resource{
		{Id: 10, AppId: 1, Code: "system:root", Name: "平台管理", Type: "menu", Component: "LAYOUT"},
		{Id: 11, AppId: 1, ParentId: 10, Code: "system:dashboard", Name: "首页", Type: "view"},
		{Id: 12, AppId: 1, ParentId: 10, Code: "system:tenant", Name: "租户管理", Type: "menu", Component: "LAYOUT"},
		{Id: 13, AppId: 1, ParentId: 12, Code: "system:tenant:query", Name: "查询", Type: "action"},
		{Id: 20, AppId: 2, Code: "basic:root", Name: "企业管理", Type: "menu", Component: "LAYOUT"},
		{Id: 21, AppId: 2, ParentId: 20, Code: "basic:user", Name: "用户中心", Type: "menu", Component: "LAYOUT"},
		{Id: 22, AppId: 2, ParentId: 21, Code: "basic:user:role", Name: "角色管理", Type: "menu"},
		// A real menu may have the same name or a code ending in :root.
		{Id: 23, AppId: 2, ParentId: 20, Code: "basic:reports:root", Name: "企业管理", Type: "menu", Component: "LAYOUT"},
	}
	items := []*pb.Resource{}
	for _, resource := range resources {
		if (q.TargetAppId == 0 || q.TargetAppId == resource.AppId) && (q.Id == 0 || q.Id == resource.Id) {
			items = append(items, resource)
		}
	}
	return &pb.QueryReply{Total: int64(len(items)), Resources: items}, nil
}

func TestAuthorizationCatalogStartsWithMenusInsideEachApplication(t *testing.T) {
	h := New(&authorizationCatalogPlatform{}, Config{})
	for _, endpoint := range []string{"getTenantResources", "getGlobalResource"} {
		t.Run(endpoint, func(t *testing.T) {
			out := request(t, h, "GET", "/system/app/"+endpoint, "")
			if out.num("code") != 0 {
				t.Fatalf("resource catalog failed: %#v", out)
			}
			for _, app := range []struct {
				id      string
				name    string
				ids     []int64
				menus   []int64
				parent  int
				childID int64
			}{
				{"1", "平台管理", []int64{11, 12, 13}, []int64{11, 12}, 1, 13},
				{"2", "企业管理", []int64{21, 22, 23}, []int64{21, 23}, 0, 22},
			} {
				group := out.obj("data").obj(app.id)
				if group.str("name") != app.name || !reflect.DeepEqual(group.arr("ids"), app.ids) {
					t.Fatalf("group heading or selectable resource IDs are wrong: %#v", group)
				}
				menus := group["resources"].([]any)
				if len(menus) != len(app.menus) {
					t.Fatalf("application wrapper still present: %#v", menus)
				}
				for i, id := range app.menus {
					if object(menus[i]).num("id") != id {
						t.Fatalf("real menu was dropped or reordered: %#v", menus)
					}
				}
				children := object(menus[app.parent])["children"].([]any)
				if len(children) != 1 || object(children[0]).num("id") != app.childID {
					t.Fatalf("nested permissions were changed: %#v", children)
				}
			}
		})
	}

	// Resource maintenance still needs the actual routing hierarchy.
	out := request(t, h, "GET", "/system/app/getAppResources?app_id=2", "")
	resources := out["data"].([]any)
	if len(resources) != 1 || object(resources[0]).num("id") != 20 {
		t.Fatal("authorization presentation changed the resource maintenance tree")
	}
	out = request(t, h, "GET", "/system/app/getResource?id=20", "")
	if out.obj("data").str("code") != "basic:root" {
		t.Fatal("routing root is no longer available for resource maintenance")
	}
}

type roleAuthorizationPlatform struct {
	mockPlatform
	administrator bool
}

func (m *roleAuthorizationPlatform) Query(ctx context.Context, q *pb.QueryRequest, opts ...client.CallOption) (*pb.QueryReply, error) {
	return &pb.QueryReply{Total: 1, Roles: []*pb.Role{{
		Id: q.Id, Administrator: m.administrator,
		Grants: []*pb.ResourceGrant{{AppId: 2, ResourceId: 21, DataScope: 2}},
	}}}, nil
}

func TestRoleAuthorizationIncludesAdministratorProtection(t *testing.T) {
	for _, administrator := range []bool{true, false} {
		out := request(t, New(&roleAuthorizationPlatform{administrator: administrator}, Config{}),
			"GET", "/system/role/queryRoleResources?role_id=7", "")
		data := out.obj("data")
		if out.num("code") != 0 || data["administrator"] != administrator {
			t.Fatalf("role protection state missing: %#v", out)
		}
		if !reflect.DeepEqual(data.obj("resource_ids").arr("2"), []int64{21}) ||
			data.obj("resource_map").obj("2").num("21") != 2 {
			t.Fatalf("read-only role grants must remain visible with their data scopes: %#v", data)
		}
	}
}
