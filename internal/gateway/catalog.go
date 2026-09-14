package gateway

import (
	"context"
	"encoding/json"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/saas/domain/fault"
	"sort"
	"strconv"
	"strings"
)

func resourceObject(v *pb.Resource, navigation bool) payload {
	x := object(v)
	meta := payload{}
	_ = json.Unmarshal([]byte(v.MetaJson), &meta)
	meta["title"] = v.Name
	meta["icon"] = v.Icon
	x["meta"] = meta
	x["key"] = v.Id
	x["title"] = v.Name
	x["open_with"] = v.OpenWith
	if v.OpenWith == "" {
		x["open_with"] = "component"
	}
	if navigation {
		x["name"] = strings.ReplaceAll(v.Code, ":", "_")
		if strings.Contains(v.Path, "/:") {
			meta["hideMenu"] = true
		}
	}
	return x
}
func resourceTree(resources []*pb.Resource, navigation bool) []any {
	nodes := []any{}
	for _, v := range resources {
		if navigation && (v.OpenWith == "outside" || (v.Type != "menu" && v.Type != "view")) {
			continue
		}
		nodes = append(nodes, resourceObject(v, navigation))
	}
	return genericTree(nodes)
}

func authorizationResources(app *pb.App, resources []*pb.Resource) []*pb.Resource {
	items := make([]*pb.Resource, 0, len(resources))
	for _, resource := range resources {
		// The application is already the group heading. Its layout root is
		// structural, not a permission; Auth restores ancestors for routing.
		if resource.Code == app.Code+":root" && resource.ParentId == 0 &&
			resource.Type == "menu" && resource.Component == "LAYOUT" {
			continue
		}
		items = append(items, resource)
	}
	return items
}

func genericTree(items []any) []any {
	nodes := map[int64]payload{}
	order := []int64{}
	for _, v := range items {
		x := object(v)
		id := x.num("id")
		nodes[id] = x
		order = append(order, id)
	}
	out := []any{}
	for _, id := range order {
		x := nodes[id]
		pid := x.num("parent_id")
		if p, ok := nodes[pid]; ok && pid != id {
			children, _ := p["children"].([]any)
			p["children"] = append(children, x)
		} else {
			out = append(out, x)
		}
	}
	return out
}
func (g *Gateway) registerCatalog() {
	for _, path := range []string{"getGlobalResource", "getTenantResources", "getAppResources", "getResource"} {
		g.add("GET", "/system/app/"+path, true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
			q := queryContext(c, p, pb.Kind_RESOURCES)
			if path == "getResource" && q.Id <= 0 {
				return nil, fault.Invalid("必须指定资源ID")
			}
			q.Page = 0
			q.PageSize = 1000
			q.ForAuthorization = path == "getTenantResources"
			r, e := g.queryAll(ctx, q)
			if e != nil {
				return nil, e
			}
			if path == "getResource" {
				if len(r.Resources) == 0 {
					return nil, fault.NotFound
				}
				return resourceObject(r.Resources[0], false), nil
			}
			if path == "getAppResources" {
				return resourceTree(r.Resources, false), nil
			}
			apps, e := g.queryAll(ctx, &pb.QueryRequest{Context: c, Kind: pb.Kind_APPS, AvailableOnly: path == "getTenantResources", TargetTenantId: q.TargetTenantId, PageSize: 1000})
			if e != nil {
				return nil, e
			}
			grouped := map[int64][]*pb.Resource{}
			for _, v := range r.Resources {
				grouped[v.AppId] = append(grouped[v.AppId], v)
			}
			out := map[string]any{}
			for _, a := range apps.Apps {
				rs := authorizationResources(a, grouped[a.Id])
				if len(rs) == 0 && a.Type != "third" {
					continue
				}
				ids := []int64{}
				for _, v := range rs {
					ids = append(ids, v.Id)
				}
				header := appObject(a)
				header["ids"] = ids
				header["resources"] = resourceTree(rs, false)
				out[strconv.FormatInt(a.Id, 10)] = header
			}
			return out, nil
		})
	}
	g.add("GET", "/system/app/queryResourceApis", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		q := queryContext(c, p, pb.Kind_OPERATIONS)
		q.ParentId = p.num("resource_id")
		r, e := g.queryAll(ctx, q)
		if e != nil {
			return nil, e
		}
		items := []any{}
		for _, o := range r.Operations {
			items = append(items, operationObject(o))
		}
		return payload{"items": items, "total": r.Total}, nil
	})
	g.add("GET", "/app/info/servers", false, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		r, e := g.queryAll(ctx, &pb.QueryRequest{Context: c, Kind: pb.Kind_APPS, PageSize: 1000})
		if e != nil {
			return nil, e
		}
		out := []any{}
		for _, a := range r.Apps {
			entry := appObject(a)
			entry["name"] = a.Code
			out = append(out, entry)
		}
		return out, nil
	})
	g.add("GET", "/app/info/routes", false, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		a, e := g.queryAll(ctx, &pb.QueryRequest{Context: c, Kind: pb.Kind_APPS, PageSize: 1000})
		if e != nil {
			return nil, e
		}
		id := int64(0)
		for _, v := range a.Apps {
			if v.Code == p.str("server") {
				id = v.Id
			}
		}
		if id == 0 {
			return nil, fault.Invalid("服务未注册")
		}
		r, e := g.queryAll(ctx, &pb.QueryRequest{Context: c, Kind: pb.Kind_OPERATIONS, TargetAppId: id, PageSize: 1000})
		if e != nil {
			return nil, e
		}
		groups := map[string][]any{}
		for _, o := range r.Operations {
			groups[o.Group] = append(groups[o.Group], operationObject(o))
		}
		names := []string{}
		for k := range groups {
			names = append(names, k)
		}
		sort.Strings(names)
		classes := []any{}
		for _, n := range names {
			classes = append(classes, payload{"name": n})
		}
		return payload{"classes": classes, "actions": groups}, nil
	})
	g.add("GET", "/system/app/removeResource", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		r, e := g.client.Delete(ctx, &pb.DeleteRequest{Context: c, Kind: pb.Kind_RESOURCES, Id: p.num("resource_id", "id")})
		return r.GetSuccess(), e
	})
	g.add("POST", "/system/app/saveResource", true, func(ctx context.Context, c *pb.Context, in payload) (any, error) {
		p := normalize(in)
		if v, ok := p["meta"]; ok {
			if s, ok := v.(string); ok {
				p["meta_json"] = s
			} else {
				b, _ := json.Marshal(v)
				p["meta_json"] = string(b)
			}
		}
		v := &pb.Resource{}
		if e := p.decode(v); e != nil {
			return nil, e
		}
		if apis, ok := p["apis"].([]any); ok {
			for _, raw := range apis {
				a := object(raw)
				v.Operations = append(v.Operations, &pb.Operation{Id: a.num("id"), AppId: v.AppId, OperationId: a.str("operation_id"), Method: a.str("method"), Path: a.str("uri", "path"), Group: a.str("controller", "group"), Action: a.str("action")})
			}
		}
		r, e := g.client.Save(ctx, &pb.SaveRequest{Context: c, Entity: &pb.SaveRequest_Resource{Resource: v}})
		if e != nil {
			return nil, e
		}
		return r.Id, nil
	})
	g.add("GET", "/system/app/queryTeantAuthorizes", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		r, e := g.client.Query(ctx, queryContext(c, p, pb.Kind_TENANT_APPS))
		if e != nil {
			return nil, e
		}
		return payload{"items": empty(r.TenantApps), "total": r.Total}, nil
	})
	g.add("GET", "/system/app/getTenantResourceIds", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		q := queryContext(c, p, pb.Kind_TENANT_APPS)
		q.Page = 0
		q.PageSize = 1000
		if q.TargetTenantId == 0 {
			q.TargetTenantId = c.TenantId
		}
		r, e := g.queryAll(ctx, q)
		if e != nil {
			return nil, e
		}
		out := map[string][]int64{}
		for _, v := range r.TenantApps {
			out[strconv.FormatInt(v.AppId, 10)] = empty(v.ResourceIds)
		}
		return out, nil
	})
}
func operationObject(o *pb.Operation) payload {
	return payload{"id": o.Id, "app_id": o.AppId, "resource_id": o.ResourceId, "operation_id": o.OperationId, "controller": o.Group, "group": o.Group, "method": o.Method, "uri": o.Path, "path": o.Path, "action": o.OperationId}
}
func (g *Gateway) registerGrants() {
	g.add("POST", "/system/app/authorizeApp", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		req := &pb.EntitlementsRequest{Context: c, TenantIds: p.arr("tenant_ids")}
		for key, raw := range p.obj("resource_map") {
			aid, e := strconv.ParseInt(key, 10, 64)
			if e != nil {
				return nil, fault.Invalid("应用ID不合法")
			}
			v := object(raw)
			expires, e := parseExpiry(v["ttl"])
			if e != nil {
				return nil, e
			}
			req.Apps = append(req.Apps, &pb.AppGrant{AppId: aid, ExpiresAt: expires, ResourceIds: v.arr("ids")})
		}
		r, e := g.client.SetEntitlements(ctx, req)
		return r.GetSuccess(), e
	})
	g.add("POST", "/system/app/deauthorizeApp", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		r, e := g.client.SetEntitlements(ctx, &pb.EntitlementsRequest{Context: c, RevokeIds: p.arr("tenant_app_ids")})
		return r.GetSuccess(), e
	})
	g.add("GET", "/system/role/queryRoleResources", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		q := queryContext(c, p, pb.Kind_ROLES)
		q.Id = p.num("role_id", "id")
		r, e := g.queryAll(ctx, q)
		if e != nil {
			return nil, e
		}
		if len(r.Roles) != 1 {
			return nil, fault.NotFound
		}
		ids := map[string][]int64{}
		scopes := map[string]map[string]int32{}
		for _, v := range r.Roles[0].Grants {
			aid := strconv.FormatInt(v.AppId, 10)
			ids[aid] = append(ids[aid], v.ResourceId)
			if scopes[aid] == nil {
				scopes[aid] = map[string]int32{}
			}
			scopes[aid][strconv.FormatInt(v.ResourceId, 10)] = v.DataScope
		}
		return payload{"resource_ids": ids, "resource_map": scopes, "administrator": r.Roles[0].Administrator}, nil
	})
	g.add("POST", "/system/role/authRoleResource", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		req := &pb.RoleResourcesRequest{Context: c, TenantId: p.num("tenant_id"), RoleId: p.num("role_id")}
		for key, raw := range p.obj("resource_map") {
			aid, e := strconv.ParseInt(key, 10, 64)
			if e != nil {
				return nil, fault.Invalid("应用ID不合法")
			}
			v := object(raw)
			scopes := v.obj("scope")
			for _, rid := range v.arr("ids") {
				scope := int32(5)
				if raw, ok := scopes[strconv.FormatInt(rid, 10)]; ok {
					scope = int32(payload{"s": raw}.num("s"))
				}
				req.Grants = append(req.Grants, &pb.ResourceGrant{AppId: aid, ResourceId: rid, DataScope: scope})
			}
		}
		r, e := g.client.SetRoleResources(ctx, req)
		return r.GetSuccess(), e
	})
	g.add("POST", "/system/role/relateEmployee", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		action := p.str("action")
		if action != "" && action != "add" && action != "remove" && action != "delete" && action != "del" {
			return nil, fault.Invalid("角色成员操作不支持")
		}
		r, e := g.client.SetRoleMembers(ctx, &pb.RoleMembersRequest{Context: c, TenantId: p.num("tenant_id"), RoleId: p.num("role_id"), UserIds: p.arr("user_ids"), Remove: action == "remove" || action == "delete" || action == "del"})
		return r.GetSuccess(), e
	})
}

func menuTree(resources []*pb.Resource) []any {
	nodes := []any{}
	for _, v := range resources {
		if (v.Type != "menu" && v.Type != "view") || strings.Contains(v.Path, "/:") {
			continue
		}
		x := resourceObject(v, false)
		if x.obj("meta").boolean("hideMenu") {
			continue
		}
		nodes = append(nodes, x)
	}
	tree := genericTree(nodes)
	// The application root is a routing container (for example
	// `basic:root`), not a navigable menu item. Keep it in `routes` so the
	// layout hierarchy remains intact, but expose its children directly in
	// `menus` so the application itself is not rendered as a first-level menu.
	menus := []any{}
	for _, item := range tree {
		x := object(item)
		if strings.HasSuffix(x.str("code"), ":root") {
			children, _ := x["children"].([]any)
			menus = append(menus, children...)
			continue
		}
		menus = append(menus, item)
	}
	return menus
}

// Publication is catalog metadata; tenant entitlements still determine availability.
func appObject(v *pb.App) payload {
	x := object(v)
	x["icon"] = v.Icon
	x["desc"] = v.Description
	x["remark"] = v.Remark
	x["tenant_app_id"] = v.TenantAppId
	x["type"] = v.Type
	if v.Type == "" {
		x["type"] = "self"
	}
	x["url"] = v.Url
	x["is_public"] = v.IsPublic
	x["expiration_time"] = v.ExpiresAt
	return x
}
func appObjects(apps []*pb.App) []any {
	out := []any{}
	for _, v := range apps {
		out = append(out, appObject(v))
	}
	return out
}
