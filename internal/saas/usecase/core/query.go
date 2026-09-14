package core

import (
	"context"
	"encoding/json"
	"kerthus/internal/saas/domain/access"
	"kerthus/internal/saas/domain/audit"
	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/dictionary"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
	"strings"
)

func maskAgentProviders(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return raw
	}
	for _, item := range items {
		for key := range item {
			lower := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
			if lower == "api_key" || lower == "apikey" || lower == "access_key" || lower == "accesskey" {
				item[key] = "********"
			}
		}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return raw
	}
	return string(b)
}

func resourceDTO(v catalog.Resource) *c.Resource {
	return &c.Resource{ID: v.ID, AppID: v.AppID, ParentID: v.ParentID, Code: v.Code, Name: v.Name, Type: v.Type, Path: v.Path, Component: v.Component, Redirect: v.Redirect, OpenWith: v.OpenWith, Remark: v.Remark, Icon: v.Icon, MetaJSON: v.MetaJSON, Status: ptr(v.Status), Sort: v.Sort, IsPublic: v.IsPublic, IsDataAccess: v.IsDataAccess}
}
func appDTO(v catalog.App) *c.App {
	return &c.App{Type: catalog.AppType(v.Type), URL: v.URL, IsPublic: v.IsPublic, ID: v.ID, Code: v.Code, Name: v.Name, Icon: v.Icon, Description: v.Description, Remark: v.Remark, Version: v.Version, Status: ptr(v.Status), ServiceKey: v.ServiceKey, RoutePrefix: v.RoutePrefix, Home: v.Home, FrontendEntry: v.FrontendEntry, Managed: v.Managed}
}
func orgDTO(v organization.Org) *c.Org {
	return &c.Org{ID: v.ID, TenantID: v.TenantID, ParentID: v.ParentID, UnitID: v.UnitID, Type: v.Type, Name: v.Name, ShortName: v.ShortName, Status: ptr(v.Status), Sort: v.Sort, Remark: v.Remark}
}
func positionDTO(v organization.Position) *c.Position {
	return &c.Position{ID: v.ID, TenantID: v.TenantID, OrgID: v.OrgID, Name: v.Name, Status: ptr(v.Status), Remark: v.Remark, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func tenantDTO(v tenant.Tenant, includeAgentCredentials bool) *c.Tenant {
	key, providers := "", ""
	if includeAgentCredentials {
		key, providers = v.AgentAPIKey, v.AgentProviders
	} else {
		if v.AgentAPIKey != "" {
			key = "********"
		}
		providers = maskAgentProviders(v.AgentProviders)
	}
	return &c.Tenant{ID: v.ID, Name: v.Name, Logo: v.Logo, ContactPerson: v.ContactPerson, ContactPhone: v.ContactPhone, ContactEmail: v.ContactEmail, CreditCode: v.CreditCode, AddressJSON: v.AddressJSON, AddressDetail: v.AddressDetail, Description: v.Description, Status: ptr(v.Status), VerifyStatus: v.VerifyStatus, ExpiresAt: v.ExpiresAt, CreatedAt: v.CreatedAt, AgentProvider: v.AgentProvider, AgentModel: v.AgentModel, AgentEndpoint: v.AgentEndpoint, AgentAPIKey: key, AgentEnabled: v.AgentEnabled, AgentProviders: providers}
}
func opDTO(v catalog.Operation) *c.Operation {
	return &c.Operation{ID: v.ID, AppID: v.AppID, ResourceID: v.ResourceID, OperationID: v.OperationID, Method: v.Method, Path: v.Path, Group: v.GroupName, Action: v.Action}
}
func (s *Service) memberDTO(u ports.Unit, m tenant.Member) (*c.Member, error) {
	var user identity.User
	if e := u.Get(ports.Users, m.UserID, &user); e != nil {
		return nil, e
	}
	out := &c.Member{User: userDTO(user), TenantID: m.TenantID, AppID: m.DefaultAppID, Status: ptr(m.Status)}
	orgs, e := all[organization.MemberOrg](u, ports.MemberOrgs, filter("tenant_id", m.TenantID, "user_id", m.UserID))
	if e != nil {
		return nil, e
	}
	for _, link := range orgs {
		var org organization.Org
		if e = u.Get(ports.Orgs, link.OrgID, &org); e != nil {
			return nil, e
		}
		out.OrgIDs = append(out.OrgIDs, org.ID)
		out.Orgs = append(out.Orgs, orgDTO(org))
	}
	positions, e := all[organization.MemberPosition](u, ports.MemberPositions, filter("tenant_id", m.TenantID, "user_id", m.UserID))
	if e != nil {
		return nil, e
	}
	for _, link := range positions {
		var p organization.Position
		if e = u.Get(ports.Positions, link.PositionID, &p); e != nil {
			return nil, e
		}
		out.PositionIDs = append(out.PositionIDs, p.ID)
		out.Positions = append(out.Positions, positionDTO(p))
	}
	return out, nil
}
func (s *Service) Query(ctx context.Context, req *c.QueryRequest) (*c.QueryReply, error) {
	out := &c.QueryReply{}
	if req == nil {
		return nil, fault.Invalid("缺少查询参数")
	}
	if req.Page < 0 || req.PageSize < 0 || req.PageSize > 1000 || req.Page > 100000 {
		return nil, fault.Invalid("分页参数超出范围")
	}
	if req.OrgID < 0 || len(req.Name) > 191 || len(req.Phone) > 191 || len(req.Search) > 191 {
		return nil, fault.Invalid("查询条件不合法")
	}
	if req.CreatedFrom < 0 || req.CreatedBefore < 0 || (req.CreatedFrom > 0 && req.CreatedBefore > 0 && req.CreatedFrom >= req.CreatedBefore) {
		return nil, fault.Invalid("创建时间范围不合法")
	}
	if req.Kind != c.KindTenants && (req.CreatedFrom != 0 || req.CreatedBefore != 0) {
		return nil, fault.Invalid("该列表不支持创建时间筛选")
	}
	e := s.DB.Read(ctx, func(u ports.Unit) error {
		needApp := !(req.Kind == c.KindApps && req.AvailableOnly) && req.Kind != c.KindDistricts
		a, e := s.resolve(ctx, u, req.Context, needApp)
		if e != nil {
			return e
		}
		tid, e := targetTenant(a, req.TargetTenantID)
		if e != nil {
			return e
		}
		f := ports.Filter{Equal: map[string]any{}, Order: req.Sort, Desc: req.Desc, Page: int(req.Page), PageSize: int(req.PageSize), Search: strings.TrimSpace(req.Search)}
		if f.Order == "" && (req.Kind == c.KindOrgs || req.Kind == c.KindResources) {
			f.Order = "sort"
		}
		identityFilter := ports.Filter{Search: f.Search, Contains: map[string]string{}}
		if name := strings.TrimSpace(req.Name); name != "" {
			identityFilter.Contains["name"] = name
		}
		if phone := strings.TrimSpace(req.Phone); phone != "" {
			identityFilter.Contains["phone"] = phone
		}
		if f.PageSize == 0 {
			f.PageSize = 1000
		}
		if req.ID > 0 {
			f.Equal["id"] = req.ID
		}
		scoped := func(action string) error {
			if e := s.require(u, a, action); e != nil {
				return e
			}
			f.Equal["tenant_id"] = tid
			return nil
		}
		count := func(table ports.Table) error { var err error; out.Total, err = u.Count(table, f); return err }
		switch req.Kind {
		case c.KindUsers:
			if e = requirePlatform(a); e != nil {
				return e
			}
			f.Contains = identityFilter.Contains
			items, e := all[identity.User](u, ports.Users, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				out.Users = append(out.Users, userDTO(v))
			}
			return count(ports.Users)
		case c.KindTenants:
			if e = requirePlatform(a); e != nil {
				return e
			}
			if req.AvailableOnly {
				f.Equal["status"], f.Equal["verify_status"] = 1, 1
			}
			if req.CreatedFrom > 0 {
				f.GreaterEqual = map[string]int64{"created_at": req.CreatedFrom}
			}
			if req.CreatedBefore > 0 {
				f.LessThan = map[string]int64{"created_at": req.CreatedBefore}
			}
			items, e := all[tenant.Tenant](u, ports.Tenants, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				out.Tenants = append(out.Tenants, tenantDTO(v, req.IncludeAgentCredentials))
			}
			return count(ports.Tenants)
		case c.KindMembers:
			if e = scoped("membership.read"); e != nil {
				return e
			}
			if req.ID > 0 {
				delete(f.Equal, "id")
				f.Equal["user_id"] = req.ID
			}
			f.Search = ""
			if identityFilter.Search != "" || len(identityFilter.Contains) > 0 {
				users, e := all[identity.User](u, ports.Users, identityFilter)
				if e != nil {
					return e
				}
				ids := []int64{}
				for _, v := range users {
					ids = append(ids, v.ID)
				}
				f.In = map[string][]int64{"user_id": ids}
			}
			if req.OrgID > 0 {
				orgIDs, e := organizationIDs(u, tid, req.OrgID, req.IncludeChildren)
				if e != nil {
					return e
				}
				orgFilter := eq("tenant_id", tid)
				orgFilter.In = map[string][]int64{"org_id": orgIDs}
				links, e := all[organization.MemberOrg](u, ports.MemberOrgs, orgFilter)
				if e != nil {
					return e
				}
				ids := []int64{}
				for _, link := range links {
					ids = append(ids, link.UserID)
				}
				intersectIDs(&f, "user_id", ids)
			}
			roleLinks := map[int64]bool{}
			if req.RoleID > 0 {
				var role access.Role
				if e = u.Get(ports.Roles, req.RoleID, &role); e != nil {
					return e
				}
				if role.TenantID != tid {
					return fault.Forbidden
				}
				links, e := all[access.RoleMember](u, ports.RoleMembers, filter("tenant_id", tid, "role_id", role.ID))
				if e != nil {
					return e
				}
				for _, v := range links {
					roleLinks[v.UserID] = true
				}
			}
			if req.ForAuthorization && req.RoleID > 0 {
				ids := []int64{}
				if searched, ok := f.In["user_id"]; ok {
					for _, id := range searched {
						if roleLinks[id] {
							ids = append(ids, id)
						}
					}
				} else {
					for id := range roleLinks {
						ids = append(ids, id)
					}
				}
				if f.In == nil {
					f.In = map[string][]int64{}
				}
				f.In["user_id"] = ids
			}
			items, e := all[tenant.Member](u, ports.Members, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				dto, e := s.memberDTO(u, v)
				if e != nil {
					return e
				}
				dto.HasAdd = roleLinks[v.UserID]
				out.Members = append(out.Members, dto)
			}
			return count(ports.Members)
		case c.KindOrgs:
			if e = scoped("organization.read"); e != nil {
				return e
			}
			items, e := all[organization.Org](u, ports.Orgs, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				out.Orgs = append(out.Orgs, orgDTO(v))
			}
			return count(ports.Orgs)
		case c.KindPositions:
			if e = scoped("organization.read"); e != nil {
				return e
			}
			if req.AvailableOnly {
				f.Equal["status"] = 1
			}
			if req.OrgID > 0 {
				orgIDs, e := organizationIDs(u, tid, req.OrgID, req.IncludeChildren)
				if e != nil {
					return e
				}
				f.In = map[string][]int64{"org_id": orgIDs}
			}
			items, e := all[organization.Position](u, ports.Positions, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				dto := positionDTO(v)
				var org organization.Org
				if e = u.Get(ports.Orgs, v.OrgID, &org); e != nil {
					return e
				}
				dto.OrgName = org.Name
				out.Positions = append(out.Positions, dto)
			}
			return count(ports.Positions)
		case c.KindApps:
			if req.AvailableOnly {
				if tid != a.Tenant.ID && !a.User.PlatformAdmin {
					return fault.Forbidden
				}
				apps, e := s.availableApps(u, tid, a.User.PlatformAdmin && tid == a.Tenant.ID && !req.TenantAppsOnly)
				if e != nil {
					return e
				}
				ids := []int64{}
				for _, v := range apps {
					ids = append(ids, v.ID)
				}
				f.In = map[string][]int64{"id": ids}
				if code := strings.TrimSpace(req.Code); code != "" {
					f.Contains = map[string]string{"code": code}
				}
				apps, e = all[catalog.App](u, ports.Apps, f)
				if e != nil {
					return e
				}
				entitlements, e := all[catalog.Entitlement](u, ports.Entitlements, eq("tenant_id", tid))
				if e != nil {
					return e
				}
				byApp := map[int64]catalog.Entitlement{}
				for _, entitlement := range entitlements {
					byApp[entitlement.AppID] = entitlement
				}
				for _, app := range apps {
					dto := appDTO(app)
					entitlement := byApp[app.ID]
					dto.ExpiresAt, dto.TenantAppID = entitlement.ExpiresAt, entitlement.ID
					out.Apps = append(out.Apps, dto)
				}
				return count(ports.Apps)
			}
			if e = requirePlatform(a); e != nil {
				return e
			}
			if code := strings.TrimSpace(req.Code); code != "" {
				f.Contains = map[string]string{"code": code}
			}
			items, e := all[catalog.App](u, ports.Apps, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				out.Apps = append(out.Apps, appDTO(v))
			}
			return count(ports.Apps)
		case c.KindResources:
			if req.ForAuthorization {
				if e = s.require(u, a, "access.read"); e != nil {
					return e
				}
				if !a.User.PlatformAdmin || req.TargetTenantID > 0 {
					ents, e := all[catalog.TenantResource](u, ports.TenantResources, eq("tenant_id", tid))
					if e != nil {
						return e
					}
					ids := []int64{}
					apps, e := s.availableApps(u, tid, false)
					if e != nil {
						return e
					}
					valid := map[int64]bool{}
					for _, app := range apps {
						valid[app.ID] = true
					}
					for _, v := range ents {
						if valid[v.AppID] {
							ids = append(ids, v.ResourceID)
						}
					}
					f.In = map[string][]int64{"id": ids}
					f.Equal["status"] = 1
				}
			} else if e = requirePlatform(a); e != nil {
				return e
			}
			if req.TargetAppID > 0 {
				f.Equal["app_id"] = req.TargetAppID
			}
			items, e := all[catalog.Resource](u, ports.Resources, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				out.Resources = append(out.Resources, resourceDTO(v))
			}
			return count(ports.Resources)
		case c.KindOperations:
			if e = requirePlatform(a); e != nil {
				return e
			}
			if req.ParentID > 0 {
				f.Equal["resource_id"] = req.ParentID
			}
			if req.TargetAppID > 0 {
				f.Equal["app_id"] = req.TargetAppID
			}
			items, e := all[catalog.Operation](u, ports.Operations, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				out.Operations = append(out.Operations, opDTO(v))
			}
			return count(ports.Operations)
		case c.KindTenantApps:
			if !a.User.PlatformAdmin {
				if e = scoped("access.read"); e != nil {
					return e
				}
			} else if req.TargetTenantID > 0 {
				f.Equal["tenant_id"] = tid
			}
			f.Search = ""
			if req.TargetAppID > 0 {
				f.Equal["app_id"] = req.TargetAppID
			}
			if name := strings.TrimSpace(req.TenantName); name != "" {
				tenants, err := all[tenant.Tenant](u, ports.Tenants, ports.Filter{Contains: map[string]string{"name": name}})
				if err != nil {
					return err
				}
				ids := []int64{}
				for _, item := range tenants {
					ids = append(ids, item.ID)
				}
				intersectIDs(&f, "tenant_id", ids)
			}
			if name := strings.TrimSpace(req.AppName); name != "" {
				apps, err := all[catalog.App](u, ports.Apps, ports.Filter{Contains: map[string]string{"name": name}})
				if err != nil {
					return err
				}
				ids := []int64{}
				for _, item := range apps {
					ids = append(ids, item.ID)
				}
				intersectIDs(&f, "app_id", ids)
			}
			items, e := all[catalog.Entitlement](u, ports.Entitlements, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				var app catalog.App
				var t tenant.Tenant
				if e = u.Get(ports.Apps, v.AppID, &app); e != nil {
					return e
				}
				if e = u.Get(ports.Tenants, v.TenantID, &t); e != nil {
					return e
				}
				rs, e := all[catalog.TenantResource](u, ports.TenantResources, filter("tenant_id", v.TenantID, "app_id", v.AppID))
				if e != nil {
					return e
				}
				dto := &c.TenantApp{ID: v.ID, TenantID: v.TenantID, AppID: v.AppID, TenantName: t.Name, AppName: app.Name, ExpirationTime: v.ExpiresAt, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
				for _, r := range rs {
					dto.ResourceIDs = append(dto.ResourceIDs, r.ResourceID)
				}
				out.TenantApps = append(out.TenantApps, dto)
			}
			return count(ports.Entitlements)
		case c.KindRoles:
			if e = scoped("access.read"); e != nil {
				return e
			}
			items, e := all[access.Role](u, ports.Roles, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				dto := &c.Role{ID: v.ID, TenantID: v.TenantID, Code: v.Code, Name: v.Name, Remark: v.Remark, Status: ptr(v.Status), Administrator: v.Administrator}
				grants, e := all[access.Grant](u, ports.Grants, filter("tenant_id", tid, "role_id", v.ID))
				if e != nil {
					return e
				}
				for _, g := range grants {
					dto.Grants = append(dto.Grants, &c.ResourceGrant{AppID: g.AppID, ResourceID: g.ResourceID, DataScope: g.DataScope})
				}
				out.Roles = append(out.Roles, dto)
			}
			return count(ports.Roles)
		case c.KindDistricts:
			f.Equal["parent_id"] = req.ParentID
			items, e := all[dictionary.District](u, ports.Districts, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				children, e := u.Count(ports.Districts, eq("parent_id", v.ID))
				if e != nil {
					return e
				}
				out.Districts = append(out.Districts, &c.District{ID: v.ID, ParentID: v.ParentID, Name: v.Name, HasChildren: children > 0})
			}
			return count(ports.Districts)
		case c.KindAudits:
			if e = scoped("audit.read"); e != nil {
				return e
			}
			items, e := all[audit.Event](u, ports.Audits, f)
			if e != nil {
				return e
			}
			for _, v := range items {
				out.Audits = append(out.Audits, &c.Audit{ID: v.ID, ActorID: v.ActorID, TenantID: v.TenantID, Action: v.Action, TargetID: v.TargetID, RequestID: v.RequestID, CreatedAt: v.CreatedAt})
			}
			return count(ports.Audits)
		default:
			return fault.Invalid("不支持的查询类型")
		}
	})
	if e == nil && req.ID > 0 && out.Total == 0 {
		return nil, fault.NotFound
	}
	return out, e
}

// Organization descendants are resolved from the target tenant's stored tree;
// browser-supplied children IDs are never trusted as the scope of a query.
func organizationIDs(u ports.Unit, tenantID, orgID int64, includeChildren bool) ([]int64, error) {
	var selected organization.Org
	if e := u.Get(ports.Orgs, orgID, &selected); e != nil {
		return nil, e
	}
	if selected.TenantID != tenantID {
		return nil, fault.Forbidden
	}
	ids := []int64{orgID}
	if !includeChildren {
		return ids, nil
	}
	orgs, e := all[organization.Org](u, ports.Orgs, eq("tenant_id", tenantID))
	if e != nil {
		return nil, e
	}
	children := map[int64][]int64{}
	for _, org := range orgs {
		children[org.ParentID] = append(children[org.ParentID], org.ID)
	}
	seen := map[int64]bool{orgID: true}
	for i := 0; i < len(ids); i++ {
		for _, childID := range children[ids[i]] {
			if !seen[childID] {
				seen[childID] = true
				ids = append(ids, childID)
			}
		}
	}
	return ids, nil
}

func intersectIDs(f *ports.Filter, column string, ids []int64) {
	if f.In == nil {
		f.In = map[string][]int64{}
	}
	if previous, exists := f.In[column]; exists {
		allowed := map[int64]bool{}
		for _, id := range ids {
			allowed[id] = true
		}
		ids = []int64{}
		for _, id := range previous {
			if allowed[id] {
				ids = append(ids, id)
			}
		}
	}
	f.In[column] = ids
}
