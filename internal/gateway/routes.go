package gateway

import (
	"context"
	"encoding/json"
	microerrors "go-micro.dev/v6/errors"
	"google.golang.org/protobuf/proto"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/saas/domain/fault"
	"strconv"
	"strings"
	"time"
)

func (g *Gateway) add(method, path string, check bool, fn func(context.Context, *pb.Context, payload) (any, error)) {
	g.routes[path] = route{method, fn, check}
}
func (g *Gateway) register() {
	g.add("GET", "/healthz", false, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		return g.client.Health(ctx, &pb.Empty{})
	})
	g.add("GET", "/app/info/init", false, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		return payload{"static_url": g.config.StaticURL, "mobile_url": "", "user_status": []string{"禁用", "正常"}}, nil
	})
	g.add("POST", "/system/user/login", false, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		r, e := g.client.Login(ctx, &pb.LoginRequest{Username: p.str("username"), Password: p.str("password"), Scene: p.str("scene")})
		if e != nil {
			return nil, e
		}
		v := object(r)
		v["seat_id"] = 0
		return v, nil
	})
	g.add("GET", "/system/user/logout", false, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		r, e := g.client.Logout(ctx, c)
		return r.GetSuccess(), e
	})
	g.add("GET", "/system/user/profile", false, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		user, e := g.client.Profile(ctx, c)
		return userObject(user), e
	})
	g.add("GET", "/system/user/auth", false, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		r, e := g.client.Auth(ctx, c)
		if e != nil {
			return nil, e
		}
		routes := resourceTree(r.Resources, true)
		return payload{"platform_admin": r.PlatformAdmin, "roles": empty(r.Roles), "permissions": empty(r.Permissions), "routes": routes, "menus": menuTree(r.Resources), "context": r.Context}, nil
	})
	for _, name := range []string{"modifyPassword", "resetPassword"} {
		g.add("POST", "/system/user/"+name, false, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
			r, e := g.client.ChangePassword(ctx, &pb.PasswordRequest{Context: c, UserId: p.num("user_id", "id"), OldPassword: p.str("old_password"), NewPassword: p.str("new_password", "password")})
			return r.GetSuccess(), e
		})
	}
	kinds := map[string]pb.Kind{"user": pb.Kind_USERS, "tenant": pb.Kind_TENANTS, "employee": pb.Kind_MEMBERS, "org": pb.Kind_ORGS, "position": pb.Kind_POSITIONS, "app": pb.Kind_APPS, "role": pb.Kind_ROLES, "log": pb.Kind_AUDITS}
	for group, kind := range kinds {
		// All catalog-backed business endpoints participate in the active
		// application's operation policy. Authentication endpoints below remain
		// explicitly public or session-scoped and are not agent tools.
		check := true
		g.add("GET", "/system/"+group+"/query", check, g.query(kind, "query"))
		if group == "tenant" || group == "employee" || group == "org" {
			g.add("GET", "/system/"+group+"/info", check, g.query(kind, "info"))
		}
		if group == "log" {
			continue
		}
		if group != "user" {
			g.add("POST", "/system/"+group+"/save", check, g.save(group))
			g.add("GET", "/system/"+group+"/delete", check, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
				r, e := g.client.Delete(ctx, &pb.DeleteRequest{Context: c, Kind: kind, Id: p.num("id", "role_id", "user_id"), TargetTenantId: p.num("tenant_id")})
				return r.GetSuccess(), e
			})
		}
		if group != "org" {
			g.add("GET", "/system/"+group+"/setStatus", check, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
				if _, ok := p["status"]; !ok {
					return nil, fault.Invalid("必须指定状态")
				}
				r, e := g.client.SetStatus(ctx, &pb.StatusRequest{Context: c, Kind: kind, Id: p.num("id", "user_id"), Status: int32(p.num("status")), TargetTenantId: p.num("tenant_id")})
				return r.GetSuccess(), e
			})
		}
	}
	g.add("POST", "/system/user/modifyInfo", false, g.save("user"))
	for group, kind := range map[string]pb.Kind{"tenant": pb.Kind_TENANTS, "employee": pb.Kind_MEMBERS, "position": pb.Kind_POSITIONS} {
		g.add("GET", "/system/"+group+"/getItems", true, g.query(kind, "items"))
	}
	g.add("GET", "/system/config/districts", true, g.query(pb.Kind_DISTRICTS, "list"))
	for _, path := range []string{"/system/employee/getTenantApps", "/system/employee/queryTenantApps", "/system/app/getAppItems"} {
		mode := "items"
		if strings.Contains(path, "queryTenantApps") {
			mode = "query"
		}
		protected := true
		g.add("GET", path, protected, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
			q := queryContext(c, p, pb.Kind_APPS)
			q.AvailableOnly = path != "/system/app/getAppItems"
			q.TenantAppsOnly = mode == "query"
			var r *pb.QueryReply
			var e error
			if mode == "query" {
				r, e = g.client.Query(ctx, q)
			} else {
				r, e = g.queryAll(ctx, q)
			}
			if e != nil {
				return nil, e
			}
			if mode == "items" {
				if path == "/system/app/getAppItems" {
					return itemMap(r.Apps), nil
				}
				return appObjects(r.Apps), nil
			}
			return payload{"items": appObjects(r.Apps), "total": r.Total}, nil
		})
	}
	// Availability is a public capability probe used before an application is
	// selected; it cannot be scoped to the active application's permissions.
	g.add("GET", "/system/employee/hasApp", false, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		appID := p.num("app_id")
		if appID <= 0 {
			return nil, fault.Invalid("必须指定应用ID")
		}
		apps, e := g.client.Query(ctx, &pb.QueryRequest{Context: c, Kind: pb.Kind_APPS, Id: appID, AvailableOnly: true})
		if e != nil {
			code, _ := fault.Code(e)
			if code == 404 || microerrors.FromError(e).Code == 404 {
				return false, nil
			}
			return nil, e
		}
		if len(apps.Apps) == 0 {
			return false, nil
		}
		if apps.Apps[0].Type == "third" {
			return true, nil
		}
		selected := proto.Clone(c).(*pb.Context)
		selected.AppId = appID
		r, e := g.client.Auth(ctx, selected)
		if e != nil {
			return nil, e
		}
		return len(r.Permissions) > 0, nil
	})
	g.add("POST", "/system/tenant/approve", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		r, e := g.client.ApproveTenant(ctx, &pb.ApproveRequest{Context: c, TenantId: p.num("tenant_id", "id"), Approved: p.boolean("approved") || p.num("verify_status", "status") == 1, AdminPassword: p.str("admin_password")})
		return r.GetSuccess(), e
	})
	g.add("GET", "/system/tenant/verify", true, func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		return nil, fault.Invalid("请使用POST审核接口并设置初始管理员密码")
	})
	g.registerCatalog()
	g.registerGrants()
}
func itemMap[T interface {
	GetId() int64
	GetName() string
}](items []T) map[string]string {
	out := map[string]string{}
	for _, v := range items {
		out[strconv.FormatInt(v.GetId(), 10)] = v.GetName()
	}
	return out
}
func (g *Gateway) query(kind pb.Kind, mode string) func(context.Context, *pb.Context, payload) (any, error) {
	return func(ctx context.Context, c *pb.Context, p payload) (any, error) {
		q := queryContext(c, p, kind)
		if mode == "info" && q.Id <= 0 {
			return nil, fault.Invalid("详情查询需要有效ID")
		}
		if kind == pb.Kind_TENANTS {
			if e := tenantCreatedRange(q, p); e != nil {
				return nil, e
			}
		}
		if mode != "query" {
			q.Page = 0
			q.PageSize = 1000
		}
		if mode == "items" && (kind == pb.Kind_POSITIONS || kind == pb.Kind_TENANTS) {
			q.AvailableOnly = true
		}
		q.ForAuthorization = kind == pb.Kind_MEMBERS && q.RoleId > 0 && !p.boolean("check_role")
		var r *pb.QueryReply
		var e error
		if mode != "query" || kind == pb.Kind_ORGS {
			r, e = g.queryAll(ctx, q)
		} else {
			r, e = g.client.Query(ctx, q)
		}
		if e != nil {
			return nil, e
		}
		items := []any{}
		switch kind {
		case pb.Kind_USERS:
			for _, v := range r.Users {
				items = append(items, userObject(v))
			}
		case pb.Kind_TENANTS:
			for _, v := range r.Tenants {
				x := object(v)
				// Provider API keys are needed only inside the gateway runtime;
				// never expose the raw JSON credential to the browser.
				if raw, ok := x["agent_providers"].(string); ok {
					x["agent_providers"] = maskTenantProviders(raw)
				}
				x["expiration_time"] = v.ExpiresAt
				x["register_type"] = "create"
				x["verify_status"] = v.VerifyStatus
				var addr any
				_ = json.Unmarshal([]byte(v.AddressJson), &addr)
				x["address"] = addr
				if a, ok := addr.(map[string]any); ok {
					x["address"] = a["labels"]
					x["address_code"] = a["codes"]
					x["area_code"] = a["area_code"]
				}
				x["desc"] = v.Description
				items = append(items, x)
			}
		case pb.Kind_MEMBERS:
			for _, v := range r.Members {
				x := userObject(v.User)
				x["status"] = v.GetStatus()
				x["tenant_id"] = v.TenantId
				x["app_id"] = v.AppId
				x["org_ids"] = empty(v.OrgIds)
				x["position_ids"] = empty(v.PositionIds)
				x["orgs"] = empty(v.Orgs)
				x["positions"] = empty(v.Positions)
				x["has_add"] = v.HasAdd
				items = append(items, x)
			}
		case pb.Kind_ORGS:
			for _, v := range r.Orgs {
				x := object(v)
				x["key"] = v.Id
				x["value"] = v.Id
				x["title"] = v.Name
				items = append(items, x)
			}
		case pb.Kind_POSITIONS:
			for _, v := range r.Positions {
				items = append(items, v)
			}
		case pb.Kind_APPS:
			for _, v := range r.Apps {
				items = append(items, appObject(v))
			}
		case pb.Kind_ROLES:
			for _, v := range r.Roles {
				items = append(items, v)
			}
		case pb.Kind_DISTRICTS:
			for _, v := range r.Districts {
				items = append(items, payload{"id": v.Id, "parent_id": v.ParentId, "name": v.Name, "value": v.Id, "label": v.Name, "has_children": v.HasChildren, "area_code": v.Id})
			}
		case pb.Kind_AUDITS:
			for _, v := range r.Audits {
				items = append(items, v)
			}
		}
		if mode == "info" {
			if len(items) == 0 {
				return nil, fault.NotFound
			}
			return items[0], nil
		}
		if mode == "items" {
			if kind == pb.Kind_POSITIONS {
				return items, nil
			}
			m := map[string]any{}
			for _, v := range items {
				x := object(v)
				m[strconv.FormatInt(x.num("id"), 10)] = x["name"]
			}
			return m, nil
		}
		if mode == "list" {
			return items, nil
		}
		if kind == pb.Kind_ORGS {
			return genericTree(items), nil
		}
		return payload{"items": items, "total": r.Total}, nil
	}
}
func (g *Gateway) save(group string) func(context.Context, *pb.Context, payload) (any, error) {
	return func(ctx context.Context, c *pb.Context, in payload) (any, error) {
		p := normalize(in)
		r := &pb.SaveRequest{Context: c, Password: p.str("password")}
		var e error
		switch group {
		case "user":
			v := &pb.User{}
			e = p.decode(v)
			r.Entity = &pb.SaveRequest_User{User: v}
		case "tenant":
			v := &pb.Tenant{}
			if a, ok := p["address"]; ok {
				if text, ok := a.(string); ok {
					if e := json.Unmarshal([]byte(text), &a); e != nil {
						return nil, fault.Invalid("地区格式错误")
					}
				}
				b, _ := json.Marshal(payload{"labels": a, "codes": p["address_code"], "area_code": p["area_code"]})
				p["address_json"] = string(b)
			}
			if desc, ok := p["desc"]; ok {
				p["description"] = desc
			}
			if raw, ok := in["expiration_time"]; ok {
				n, e := parseExpiry(raw)
				if e != nil {
					return nil, e
				}
				p["expires_at"] = n
			}
			e = p.decode(v)
			r.Entity = &pb.SaveRequest_Tenant{Tenant: v}
		case "employee":
			v := &pb.Member{User: &pb.User{}}
			e = p.decode(v.User)
			v.TenantId = p.num("tenant_id")
			v.AppId = p.num("app_id")
			v.OrgIds = p.arr("org_ids")
			v.PositionIds = p.arr("position_ids")
			r.Entity = &pb.SaveRequest_Member{Member: v}
		case "org":
			v := &pb.Org{}
			e = p.decode(v)
			r.Entity = &pb.SaveRequest_Org{Org: v}
		case "position":
			v := &pb.Position{}
			e = p.decode(v)
			r.Entity = &pb.SaveRequest_Position{Position: v}
		case "app":
			v := &pb.App{}
			if desc, ok := p["desc"]; ok {
				p["description"] = desc
			}
			e = p.decode(v)
			r.Entity = &pb.SaveRequest_App{App: v}
		case "role":
			v := &pb.Role{}
			e = p.decode(v)
			r.Entity = &pb.SaveRequest_Role{Role: v}
		}
		if e != nil {
			return nil, e
		}
		out, e := g.client.Save(ctx, r)
		if e != nil {
			return nil, e
		}
		return out.Id, nil
	}
}
func parseExpiry(v any) (int64, error) {
	p := payload{"v": v}
	if n := p.num("v"); n > 0 {
		return n, nil
	}
	s := p.str("v")
	if s == "" || s == "0" {
		return 0, nil
	}
	for _, f := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		t, e := time.ParseInLocation(f, s, time.FixedZone("CST", 8*3600))
		if e == nil {
			return t.Unix(), nil
		}
	}
	return 0, fault.Invalid("有效期格式不正确")
}
