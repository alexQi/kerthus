package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"kerthus/internal/saas/domain/access"
	"kerthus/internal/saas/domain/catalog"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/domain/organization"
	"kerthus/internal/saas/domain/tenant"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,62}$`)

func (s *Service) Save(ctx context.Context, req *c.SaveRequest) (*c.SaveReply, error) {
	out := &c.SaveReply{}
	if req == nil {
		return nil, fault.Invalid("缺少保存参数")
	}
	n := 0
	for _, ok := range []bool{req.User != nil, req.Tenant != nil, req.Member != nil, req.Org != nil, req.Position != nil, req.App != nil, req.Resource != nil, req.Role != nil} {
		if ok {
			n++
		}
	}
	if n != 1 {
		return nil, fault.Invalid("每次只能保存一个实体")
	}
	e := s.DB.Write(ctx, func(u ports.Unit) error {
		a, e := s.resolve(ctx, u, req.Context, req.User == nil)
		if e != nil {
			return e
		}
		var id int64
		var action string
		switch {
		case req.User != nil:
			action = "identity.profile.save"
			id, e = s.saveUser(u, a, req.User, req.Password)
		case req.Tenant != nil:
			action = "tenant.save"
			id, e = s.saveTenant(u, a, req.Tenant)
		case req.Member != nil:
			action = "membership.write"
			id, e = s.saveMember(u, a, req.Member, req.Password)
		case req.Org != nil:
			action = "organization.save"
			id, e = s.saveOrg(u, a, req.Org)
		case req.Position != nil:
			action = "position.save"
			id, e = s.savePosition(u, a, req.Position)
		case req.App != nil:
			action = "catalog.save"
			id, e = s.saveApp(u, a, req.App)
		case req.Resource != nil:
			action = "catalog.resource.save"
			id, e = s.saveResource(u, a, req.Resource)
		case req.Role != nil:
			action = "role.save"
			id, e = s.saveRole(u, a, req.Role)
		}
		if e != nil {
			return e
		}
		out.ID = id
		return s.record(u, a, action, id, req.Context)
	})
	return out, e
}
func (s *Service) saveUser(u ports.Unit, a *actor, v *c.User, password string) (int64, error) {
	id := v.ID
	if id == 0 {
		id = a.User.ID
	}
	if id != a.User.ID && !a.User.PlatformAdmin {
		return 0, fault.Forbidden
	}
	var user identity.User
	if e := u.Get(ports.Users, id, &user); e != nil {
		return 0, e
	}
	if e := required(v.Name, "姓名"); e != nil {
		return 0, e
	}
	email := strings.TrimSpace(v.Email)
	if email != user.Email && !a.User.PlatformAdmin {
		if !verifyPassword(user.PasswordHash, password) {
			return 0, fault.Invalid("修改登录邮箱需要验证当前密码")
		}
		user.AuthVersion++
	}
	if e := ensureAvailableEmail(u, email, user); e != nil {
		return 0, e
	}
	user.Name = v.Name
	user.Email = email
	user.Avatar = v.Avatar
	user.Sex = v.Sex
	return user.ID, u.Save(ports.Users, &user)
}
func (s *Service) saveTenant(u ports.Unit, a *actor, v *c.Tenant) (int64, error) {
	if e := requirePlatform(a); e != nil {
		return 0, e
	}
	if e := required(v.Name, "租户名称"); e != nil {
		return 0, e
	}
	if v.ExpiresAt < 0 {
		return 0, fault.Invalid("有效期不合法")
	}
	if v.AddressJSON != "" && !json.Valid([]byte(v.AddressJSON)) {
		return 0, fault.Invalid("地址格式不正确")
	}
	t := tenant.Tenant{Status: 1}
	if v.ID > 0 {
		if e := u.Get(ports.Tenants, v.ID, &t); e != nil {
			return 0, e
		}
	}
	t.Name = v.Name
	t.Logo = v.Logo
	t.ContactPerson = v.ContactPerson
	t.ContactPhone = v.ContactPhone
	t.ContactEmail = v.ContactEmail
	t.CreditCode = v.CreditCode
	t.AddressJSON = v.AddressJSON
	t.AddressDetail = v.AddressDetail
	t.Description = v.Description
	t.ExpiresAt = v.ExpiresAt
	if t.AddressJSON == "" {
		t.AddressJSON = "[]"
	}
	if e := u.Save(ports.Tenants, &t); e != nil {
		return 0, e
	}
	return t.ID, nil
}
func (s *Service) saveMember(u ports.Unit, a *actor, v *c.Member, password string) (int64, error) {
	if e := s.require(u, a, "membership.write"); e != nil {
		return 0, e
	}
	tid, e := targetTenant(a, v.TenantID)
	if e != nil {
		return 0, e
	}
	if v.User == nil {
		return 0, fault.Invalid("缺少员工资料")
	}
	if e = required(v.User.Name, "姓名"); e != nil {
		return 0, e
	}
	var t tenant.Tenant
	if e = u.Get(ports.Tenants, tid, &t); e != nil {
		return 0, e
	}
	orgs := []organization.Org{}
	seen := map[int64]bool{}
	for _, id := range v.OrgIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		var org organization.Org
		if e = u.Get(ports.Orgs, id, &org); e != nil {
			return 0, e
		}
		if org.TenantID != tid || org.Status != 1 {
			return 0, fault.Invalid("组织不属于目标租户或已停用")
		}
		orgs = append(orgs, org)
	}
	positions := []organization.Position{}
	seen = map[int64]bool{}
	for _, id := range v.PositionIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		var p organization.Position
		if e = u.Get(ports.Positions, id, &p); e != nil {
			return 0, e
		}
		if p.TenantID != tid || p.Status != 1 {
			return 0, fault.Invalid("岗位不属于目标租户或已停用")
		}
		belongs := false
		for _, o := range orgs {
			if o.ID == p.OrgID {
				belongs = true
			}
		}
		if !belongs {
			return 0, fault.Invalid("岗位所属组织不在员工组织中")
		}
		positions = append(positions, p)
	}
	if v.AppID > 0 {
		apps, e := s.availableApps(u, tid, false)
		if e != nil {
			return 0, e
		}
		found := false
		for _, app := range apps {
			if app.ID == v.AppID {
				if catalog.AppType(app.Type) == "third" {
					return 0, fault.Invalid("第三方应用不能作为平台默认应用")
				}
				found = true
			}
		}
		if !found {
			// Preserve an imported or subsequently disabled default only for the
			// same existing membership. New selections never grant app access.
			member, err := first[tenant.Member](u, ports.Members, filter("tenant_id", tid, "user_id", v.User.ID))
			if err != nil && !errors.Is(err, fault.NotFound) {
				return 0, err
			}
			if v.User.ID == 0 || err != nil || member.DefaultAppID != v.AppID {
				return 0, fault.Invalid("默认应用未开通")
			}
		}
	}
	user := identity.User{}
	existing := v.User.ID > 0
	if existing {
		if e = u.Get(ports.Users, v.User.ID, &user); e != nil {
			return 0, e
		}
		if !a.User.PlatformAdmin {
			if _, e = first[tenant.Member](u, ports.Members, filter("tenant_id", tid, "user_id", user.ID)); e != nil {
				return 0, fault.Forbidden
			}
			if user.Name != v.User.Name || user.Email != v.User.Email || user.Phone != v.User.Phone {
				return 0, fault.Invalid("共享账号资料需由本人或平台管理员修改")
			}
		} else {
			user.Name = v.User.Name
			user.Avatar = v.User.Avatar
			user.Sex = v.User.Sex
			if e = u.Save(ports.Users, &user); e != nil {
				return 0, e
			}
		}
	} else {
		if strings.TrimSpace(v.User.Phone) == "" {
			return 0, fault.Invalid("手机号不能为空")
		}
		hash, e := passwordHash(password)
		if e != nil {
			return 0, e
		}
		if e := ensureAvailableEmail(u, strings.TrimSpace(v.User.Email), identity.User{}); e != nil {
			return 0, e
		}
		user = identity.User{Phone: strings.TrimSpace(v.User.Phone), Email: strings.TrimSpace(v.User.Email), Name: v.User.Name, Avatar: v.User.Avatar, Sex: v.User.Sex, Status: 1, PasswordHash: hash, SingleLogin: true, AuthVersion: 1}
		if e = u.Save(ports.Users, &user); e != nil {
			return 0, e
		}
	}
	m, e := first[tenant.Member](u, ports.Members, filter("tenant_id", tid, "user_id", user.ID))
	if e != nil && !errors.Is(e, fault.NotFound) {
		return 0, e
	}
	if errors.Is(e, fault.NotFound) {
		m = tenant.Member{TenantID: tid, UserID: user.ID, Status: 1}
		count, e := u.Count(ports.Members, eq("user_id", user.ID))
		if e != nil {
			return 0, e
		}
		m.IsDefault = count == 0
	}
	if m.Status == 0 {
		return 0, fault.Conflict("成员已停用，请通过成员状态操作恢复")
	}
	m.DefaultAppID = v.AppID
	m.DefaultOrgID = 0
	m.DefaultUnitID = 0
	if len(orgs) > 0 {
		m.DefaultOrgID = orgs[0].ID
		m.DefaultUnitID = orgs[0].UnitID
	}
	if e = u.Save(ports.Members, &m); e != nil {
		return 0, e
	}
	f := filter("tenant_id", tid, "user_id", user.ID)
	if e = u.Delete(ports.MemberOrgs, f); e != nil {
		return 0, e
	}
	if e = u.Delete(ports.MemberPositions, f); e != nil {
		return 0, e
	}
	for _, o := range orgs {
		if e = u.Save(ports.MemberOrgs, &organization.MemberOrg{TenantID: tid, UserID: user.ID, OrgID: o.ID}); e != nil {
			return 0, e
		}
	}
	for _, p := range positions {
		if e = u.Save(ports.MemberPositions, &organization.MemberPosition{TenantID: tid, UserID: user.ID, PositionID: p.ID}); e != nil {
			return 0, e
		}
	}
	return user.ID, nil
}
func (s *Service) saveOrg(u ports.Unit, a *actor, v *c.Org) (int64, error) {
	if e := s.require(u, a, "organization.write"); e != nil {
		return 0, e
	}
	tid, e := targetTenant(a, v.TenantID)
	if e != nil {
		return 0, e
	}
	if e = required(v.Name, "组织名称"); e != nil {
		return 0, e
	}
	if v.Type != "unit" && v.Type != "section" {
		return 0, fault.Invalid("组织类型必须为 unit 或 section")
	}
	org := organization.Org{TenantID: tid, Status: 1}
	if v.ID > 0 {
		if e = u.Get(ports.Orgs, v.ID, &org); e != nil {
			return 0, e
		}
		if org.TenantID != tid {
			return 0, fault.Forbidden
		}
		if org.ParentID != v.ParentID || org.Type != v.Type || (org.Status == 1 && status(v.Status, org.Status) == 0) {
			for _, table := range []ports.Table{ports.Orgs, ports.MemberOrgs, ports.Positions, ports.Members} {
				col := "org_id"
				if table == ports.Orgs {
					col = "parent_id"
				} else if table == ports.Members {
					col = "default_org_id"
				}
				n, e := u.Count(table, filter("tenant_id", tid, col, org.ID))
				if e != nil {
					return 0, e
				}
				if n > 0 {
					return 0, fault.Conflict("组织仍有子节点或引用，不能直接移动或停用")
				}
			}
		}
	}
	unitID := int64(0)
	if v.ParentID > 0 {
		parent := v.ParentID
		visited := map[int64]bool{}
		for parent > 0 {
			if parent == org.ID || visited[parent] {
				return 0, fault.Invalid("组织树不能成环")
			}
			visited[parent] = true
			var p organization.Org
			if e = u.Get(ports.Orgs, parent, &p); e != nil {
				return 0, e
			}
			if p.TenantID != tid || p.Status != 1 {
				return 0, fault.Invalid("父组织无效")
			}
			if parent == v.ParentID {
				unitID = p.UnitID
			}
			parent = p.ParentID
		}
	}
	if v.Type == "section" && unitID == 0 {
		return 0, fault.Invalid("部门必须隶属单位")
	}
	org.ParentID = v.ParentID
	org.Type = v.Type
	org.Name = v.Name
	org.ShortName = v.ShortName
	org.Sort = v.Sort
	org.Remark = v.Remark
	org.Status = status(v.Status, org.Status)
	if e = validStatus(org.Status); e != nil {
		return 0, e
	}
	org.UnitID = unitID
	if e = u.Save(ports.Orgs, &org); e != nil {
		return 0, e
	}
	if org.Type == "unit" {
		org.UnitID = org.ID
		if e = u.Save(ports.Orgs, &org); e != nil {
			return 0, e
		}
	}
	return org.ID, nil
}
func (s *Service) savePosition(u ports.Unit, a *actor, v *c.Position) (int64, error) {
	if e := s.require(u, a, "organization.write"); e != nil {
		return 0, e
	}
	tid, e := targetTenant(a, v.TenantID)
	if e != nil {
		return 0, e
	}
	if e = required(v.Name, "岗位名称"); e != nil {
		return 0, e
	}
	var org organization.Org
	if e = u.Get(ports.Orgs, v.OrgID, &org); e != nil {
		return 0, e
	}
	if org.TenantID != tid || org.Status != 1 {
		return 0, fault.Invalid("岗位组织无效")
	}
	p := organization.Position{TenantID: tid, Status: 1}
	if v.ID > 0 {
		if e = u.Get(ports.Positions, v.ID, &p); e != nil {
			return 0, e
		}
		if p.TenantID != tid {
			return 0, fault.Forbidden
		}
		if p.OrgID != v.OrgID || (p.Status == 1 && status(v.Status, p.Status) == 0) {
			n, e := u.Count(ports.MemberPositions, eq("position_id", p.ID))
			if e != nil {
				return 0, e
			}
			if n > 0 {
				return 0, fault.Conflict("岗位仍有成员，不能直接移动或停用")
			}
		}
	}
	p.OrgID = v.OrgID
	p.Name = v.Name
	p.Remark = v.Remark
	p.Status = status(v.Status, p.Status)
	if e = validStatus(p.Status); e != nil {
		return 0, e
	}
	if e = u.Save(ports.Positions, &p); e != nil {
		return 0, e
	}
	return p.ID, nil
}
func (s *Service) saveApp(u ports.Unit, a *actor, v *c.App) (int64, error) {
	if e := requirePlatform(a); e != nil {
		return 0, e
	}
	if e := required(v.Name, "应用名称"); e != nil {
		return 0, e
	}
	if !codePattern.MatchString(v.Code) {
		return 0, fault.Invalid("应用 code 格式不正确")
	}
	// UI-created catalog entries are drafts until a deployment registers the app.
	app := catalog.App{Status: 0}
	if v.ID > 0 {
		if e := u.Get(ports.Apps, v.ID, &app); e != nil {
			return 0, e
		}
		if app.Code != v.Code {
			return 0, fault.Invalid("应用 code 不可变更")
		}
	}
	if v.ServiceKey != "" || v.RoutePrefix != "" {
		if app.ServiceKey != v.ServiceKey || app.RoutePrefix != v.RoutePrefix {
			return 0, fault.Invalid("服务路由只能通过受控应用定义更新")
		}
	}
	typ := catalog.AppType(v.Type)
	if v.Type == "" && v.ID > 0 {
		typ = catalog.AppType(app.Type)
	}
	if typ != "self" && typ != "third" {
		return 0, fault.Invalid("应用类型不正确")
	}
	if v.ID > 0 && typ != catalog.AppType(app.Type) {
		return 0, fault.Invalid("应用类型创建后不可变更")
	}
	if typ == "third" {
		if app.Managed || app.ServiceKey != "" || app.RoutePrefix != "" {
			return 0, fault.Invalid("服务应用不能作为第三方链接")
		}
		if !catalog.SafeWebURL(v.URL) {
			return 0, fault.Invalid("第三方应用地址必须为有效的 HTTP(S) 地址，且不能包含账号密码")
		}
	} else if v.URL != "" {
		return 0, fault.Invalid("自建应用不使用第三方地址")
	}
	app.Type, app.URL, app.IsPublic = typ, v.URL, v.IsPublic
	app.Code = v.Code
	app.Name = v.Name
	if len(v.Icon) > 1024 || len(v.Description) > 2048 || len(v.Remark) > 2048 {
		return 0, fault.Invalid("应用展示信息过长")
	}
	app.Icon = v.Icon
	app.Description = v.Description
	app.Remark = v.Remark
	app.Version = v.Version
	if !app.Managed {
		app.Home = v.Home
		app.FrontendEntry = v.FrontendEntry
	}
	if typ == "third" {
		app.Home, app.FrontendEntry = "", ""
	}
	if e := u.Save(ports.Apps, &app); e != nil {
		return 0, e
	}
	return app.ID, nil
}
func (s *Service) saveResource(u ports.Unit, a *actor, v *c.Resource) (int64, error) {
	if e := requirePlatform(a); e != nil {
		return 0, e
	}
	if e := required(v.Name, "资源名称"); e != nil {
		return 0, e
	}
	if e := required(v.Code, "资源编码"); e != nil {
		return 0, e
	}
	if v.Type != "menu" && v.Type != "view" && v.Type != "action" && v.Type != "field" {
		return 0, fault.Invalid("资源类型不正确")
	}
	var app catalog.App
	if e := u.Get(ports.Apps, v.AppID, &app); e != nil {
		return 0, e
	}
	r := catalog.Resource{AppID: v.AppID, Status: 1}
	if v.ID > 0 {
		if e := u.Get(ports.Resources, v.ID, &r); e != nil {
			return 0, e
		}
		if r.AppID != v.AppID {
			return 0, fault.Invalid("资源不能迁移至其他应用")
		}
		if r.Code != v.Code {
			return 0, fault.Invalid("资源编码不可变更")
		}
	}
	parent := v.ParentID
	seen := map[int64]bool{}
	for parent > 0 {
		if parent == r.ID || seen[parent] {
			return 0, fault.Invalid("资源树不能成环")
		}
		seen[parent] = true
		var p catalog.Resource
		if e := u.Get(ports.Resources, parent, &p); e != nil {
			return 0, e
		}
		if p.AppID != r.AppID {
			return 0, fault.Invalid("父资源不属于当前应用")
		}
		if p.OpenWith == "outside" {
			return 0, fault.Invalid("外链资源不能包含子资源")
		}
		parent = p.ParentID
	}
	if v.MetaJSON != "" && !json.Valid([]byte(v.MetaJSON)) {
		return 0, fault.Invalid("资源 meta 必须为 JSON")
	}
	mode := v.OpenWith
	if mode == "" || mode == "route" {
		mode = "component"
	}
	switch mode {
	case "component":
		if !catalog.ComponentReference(app.Code, v.Component) {
			return 0, fault.Invalid("组件路径不合法或不属于当前应用")
		}
	case "inside", "outside":
		if v.Type != "menu" && v.Type != "view" {
			return 0, fault.Invalid("只有菜单或页面资源可以使用内外链")
		}
		if mode == "inside" {
			if !catalog.LocalAppPath(app.Code, v.Path) || !catalog.SafeWebURL(v.Component) {
				return 0, fault.Invalid("内链需要本应用的站内路径和 HTTP(S) 页面地址")
			}
		} else {
			if !catalog.SafeWebURL(v.Path) || (v.Component != "" && v.Component != "LAYOUT") || v.Redirect != "" {
				return 0, fault.Invalid("外链需要 HTTP(S) 地址且不能设置组件或重定向")
			}
			if (r.ID > 0 && r.Path == app.Home) || v.Path == app.Home {
				return 0, fault.Invalid("应用首页不能改为外链")
			}
			if r.ID > 0 {
				n, e := u.Count(ports.Resources, eq("parent_id", r.ID))
				if e != nil {
					return 0, e
				}
				if n > 0 {
					return 0, fault.Invalid("外链资源不能包含子资源")
				}
			}
		}
	default:
		return 0, fault.Invalid("资源打开方式不正确")
	}
	r.ParentID = v.ParentID
	r.Name = v.Name
	r.Code = v.Code
	r.Type = v.Type
	r.Path = v.Path
	r.Component = v.Component
	r.Redirect = strings.TrimSpace(v.Redirect)
	r.OpenWith = mode
	r.Remark = v.Remark
	r.Icon = v.Icon
	r.MetaJSON = v.MetaJSON
	if r.MetaJSON == "" {
		r.MetaJSON = "{}"
	}
	r.Sort = v.Sort
	r.IsPublic = v.IsPublic
	r.IsDataAccess = v.IsDataAccess
	r.Status = status(v.Status, r.Status)
	if e := validStatus(r.Status); e != nil {
		return 0, e
	}
	if e := u.Save(ports.Resources, &r); e != nil {
		return 0, e
	}
	// The submitted set is a full replacement, including an explicitly empty set.
	// Keep registered operations discoverable when unbound, but deny them by default.
	old, e := all[catalog.Operation](u, ports.Operations, eq("resource_id", r.ID))
	if e != nil {
		return 0, e
	}
	selected := map[int64]bool{}
	for _, op := range v.Operations {
		if op == nil {
			return 0, fault.Invalid("接口参数不能为空")
		}
		existing, e := first[catalog.Operation](u, ports.Operations, filter("app_id", app.ID, "method", op.Method, "path", op.Path))
		if e != nil {
			return 0, fault.Invalid("接口未在应用契约中注册")
		}
		if selected[existing.ID] {
			return 0, fault.Invalid("接口关联重复")
		}
		selected[existing.ID] = true
		existing.ResourceID = r.ID
		if e = u.Save(ports.Operations, &existing); e != nil {
			return 0, e
		}
	}
	for _, op := range old {
		if !selected[op.ID] {
			op.ResourceID = 0
			if e = u.Save(ports.Operations, &op); e != nil {
				return 0, e
			}
		}
	}
	return r.ID, nil
}
func (s *Service) saveRole(u ports.Unit, a *actor, v *c.Role) (int64, error) {
	if e := s.require(u, a, "access.write"); e != nil {
		return 0, e
	}
	tid, e := targetTenant(a, v.TenantID)
	if e != nil {
		return 0, e
	}
	if e = required(v.Name, "角色名称"); e != nil {
		return 0, e
	}
	r := access.Role{TenantID: tid, Status: 1}
	if v.ID > 0 {
		if e = u.Get(ports.Roles, v.ID, &r); e != nil {
			return 0, e
		}
		if r.TenantID != tid {
			return 0, fault.Forbidden
		}
	} else {
		token, e := randomToken()
		if e != nil {
			return 0, e
		}
		r.Code = fmt.Sprintf("role_%s", token[:16])
	}
	r.Name = v.Name
	r.Remark = v.Remark
	if e = u.Save(ports.Roles, &r); e != nil {
		return 0, e
	}
	return r.ID, nil
}
