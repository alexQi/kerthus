package contracts

// Transport-independent command and response types. RPC adapters map these explicitly.
type Kind int32

const (
	KindKindUnspecified Kind = 0
	KindUsers           Kind = 1
	KindTenants         Kind = 2
	KindMembers         Kind = 3
	KindOrgs            Kind = 4
	KindPositions       Kind = 5
	KindApps            Kind = 6
	KindResources       Kind = 7
	KindOperations      Kind = 8
	KindTenantApps      Kind = 9
	KindRoles           Kind = 10
	KindDistricts       Kind = 11
	KindAudits          Kind = 12
)

type Empty struct {
}
type Result struct {
	Success bool `json:"success,omitempty"`
}
type Context struct {
	Token     string `json:"token,omitempty"`
	TenantID  int64  `json:"tenant_id,omitempty"`
	AppID     int64  `json:"app_id,omitempty"`
	UnitID    int64  `json:"unit_id,omitempty"`
	SectionID int64  `json:"section_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}
type LoginRequest struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Scene    string `json:"scene,omitempty"`
}
type LoginReply struct {
	Home        string `json:"home,omitempty"`
	UserID      int64  `json:"user_id,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
	ExpiresTime int64  `json:"expires_time,omitempty"`
	TenantID    int64  `json:"tenant_id,omitempty"`
	AppID       int64  `json:"app_id,omitempty"`
	UnitID      int64  `json:"unit_id,omitempty"`
	SectionID   int64  `json:"section_id,omitempty"`
	AppCode     string `json:"app_code,omitempty"`
}
type User struct {
	// Server-owned timestamps are returned by queries and ignored by save commands.
	CreatedAt int64  `json:"created_at,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
	ID        int64  `json:"id,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	Name      string `json:"name,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
	Sex       int32  `json:"sex,omitempty"`
	Status    *int32 `json:"status,omitempty"`
}
type Tenant struct {
	ID            int64  `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	Logo          string `json:"logo,omitempty"`
	ContactPerson string `json:"contact_person,omitempty"`
	ContactPhone  string `json:"contact_phone,omitempty"`
	ContactEmail  string `json:"contact_email,omitempty"`
	CreditCode    string `json:"credit_code,omitempty"`
	AddressJSON   string `json:"address_json,omitempty"`
	AddressDetail string `json:"address_detail,omitempty"`
	Description   string `json:"description,omitempty"`
	Status        *int32 `json:"status,omitempty"`
	VerifyStatus  int32  `json:"verify_status,omitempty"`
	ExpiresAt     int64  `json:"expires_at,omitempty"`
	CreatedAt     int64  `json:"created_at,omitempty"`
}
type Member struct {
	User        *User       `json:"user,omitempty"`
	TenantID    int64       `json:"tenant_id,omitempty"`
	AppID       int64       `json:"app_id,omitempty"`
	OrgIDs      []int64     `json:"org_ids,omitempty"`
	PositionIDs []int64     `json:"position_ids,omitempty"`
	Orgs        []*Org      `json:"orgs,omitempty"`
	Positions   []*Position `json:"positions,omitempty"`
	Status      *int32      `json:"status,omitempty"`
	HasAdd      bool        `json:"has_add,omitempty"`
}
type Org struct {
	ID        int64  `json:"id,omitempty"`
	TenantID  int64  `json:"tenant_id,omitempty"`
	ParentID  int64  `json:"parent_id,omitempty"`
	UnitID    int64  `json:"unit_id,omitempty"`
	Type      string `json:"type,omitempty"`
	Name      string `json:"name,omitempty"`
	ShortName string `json:"short_name,omitempty"`
	Status    *int32 `json:"status,omitempty"`
	Sort      int32  `json:"sort,omitempty"`
	Remark    string `json:"remark,omitempty"`
}
type Position struct {
	// Server-owned timestamps are returned by queries and ignored by save commands.
	CreatedAt int64  `json:"created_at,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
	ID        int64  `json:"id,omitempty"`
	TenantID  int64  `json:"tenant_id,omitempty"`
	OrgID     int64  `json:"org_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Status    *int32 `json:"status,omitempty"`
	Remark    string `json:"remark,omitempty"`
	OrgName   string `json:"org_name,omitempty"`
}
type App struct {
	Type          string `json:"type,omitempty"`
	URL           string `json:"url,omitempty"`
	IsPublic      bool   `json:"is_public,omitempty"`
	ID            int64  `json:"id,omitempty"`
	Code          string `json:"code,omitempty"`
	Name          string `json:"name,omitempty"`
	Icon          string `json:"icon,omitempty"`
	Description   string `json:"description,omitempty"`
	Remark        string `json:"remark,omitempty"`
	Version       string `json:"version,omitempty"`
	Status        *int32 `json:"status,omitempty"`
	ServiceKey    string `json:"service_key,omitempty"`
	RoutePrefix   string `json:"route_prefix,omitempty"`
	Home          string `json:"home,omitempty"`
	FrontendEntry string `json:"frontend_entry,omitempty"`
	ExpiresAt     int64  `json:"expires_at,omitempty"`
	Managed       bool   `json:"managed,omitempty"`
	TenantAppID   int64  `json:"tenant_app_id,omitempty"`
}
type Resource struct {
	ID           int64        `json:"id,omitempty"`
	AppID        int64        `json:"app_id,omitempty"`
	ParentID     int64        `json:"parent_id,omitempty"`
	Code         string       `json:"code,omitempty"`
	Name         string       `json:"name,omitempty"`
	Type         string       `json:"type,omitempty"`
	Path         string       `json:"path,omitempty"`
	Component    string       `json:"component,omitempty"`
	Redirect     string       `json:"redirect,omitempty"`
	OpenWith     string       `json:"open_with,omitempty"`
	Remark       string       `json:"remark,omitempty"`
	Icon         string       `json:"icon,omitempty"`
	MetaJSON     string       `json:"meta_json,omitempty"`
	Status       *int32       `json:"status,omitempty"`
	Sort         int32        `json:"sort,omitempty"`
	IsPublic     bool         `json:"is_public,omitempty"`
	IsDataAccess bool         `json:"is_data_access,omitempty"`
	Operations   []*Operation `json:"operations,omitempty"`
}
type Operation struct {
	ID          int64  `json:"id,omitempty"`
	AppID       int64  `json:"app_id,omitempty"`
	ResourceID  int64  `json:"resource_id,omitempty"`
	OperationID string `json:"operation_id,omitempty"`
	Method      string `json:"method,omitempty"`
	Path        string `json:"path,omitempty"`
	Group       string `json:"group,omitempty"`
	Action      string `json:"action,omitempty"`
}
type Role struct {
	ID            int64            `json:"id,omitempty"`
	TenantID      int64            `json:"tenant_id,omitempty"`
	Code          string           `json:"code,omitempty"`
	Name          string           `json:"name,omitempty"`
	Remark        string           `json:"remark,omitempty"`
	Status        *int32           `json:"status,omitempty"`
	Administrator bool             `json:"administrator,omitempty"`
	Grants        []*ResourceGrant `json:"grants,omitempty"`
}
type ResourceGrant struct {
	AppID      int64 `json:"app_id,omitempty"`
	ResourceID int64 `json:"resource_id,omitempty"`
	DataScope  int32 `json:"data_scope,omitempty"`
}
type TenantApp struct {
	CreatedAt      int64   `json:"created_at,omitempty"`
	UpdatedAt      int64   `json:"updated_at,omitempty"`
	ID             int64   `json:"id,omitempty"`
	TenantID       int64   `json:"tenant_id,omitempty"`
	AppID          int64   `json:"app_id,omitempty"`
	TenantName     string  `json:"tenant_name,omitempty"`
	AppName        string  `json:"app_name,omitempty"`
	ExpirationTime int64   `json:"expiration_time,omitempty"`
	ResourceIDs    []int64 `json:"resource_ids,omitempty"`
}
type District struct {
	HasChildren bool   `json:"has_children,omitempty"`
	ID          int64  `json:"id,omitempty"`
	ParentID    int64  `json:"parent_id,omitempty"`
	Name        string `json:"name,omitempty"`
}
type Audit struct {
	ID        int64  `json:"id,omitempty"`
	ActorID   int64  `json:"actor_id,omitempty"`
	TenantID  int64  `json:"tenant_id,omitempty"`
	Action    string `json:"action,omitempty"`
	TargetID  int64  `json:"target_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}
type AuthReply struct {
	PlatformAdmin bool        `json:"platform_admin,omitempty"`
	Roles         []string    `json:"roles,omitempty"`
	Permissions   []string    `json:"permissions,omitempty"`
	Resources     []*Resource `json:"resources,omitempty"`
	Context       *LoginReply `json:"context,omitempty"`
}
type QueryRequest struct {
	Context          *Context `json:"context,omitempty"`
	Kind             Kind     `json:"kind,omitempty"`
	ID               int64    `json:"id,omitempty"`
	TargetTenantID   int64    `json:"target_tenant_id,omitempty"`
	TargetAppID      int64    `json:"target_app_id,omitempty"`
	ParentID         int64    `json:"parent_id,omitempty"`
	RoleID           int64    `json:"role_id,omitempty"`
	Page             int32    `json:"page,omitempty"`
	PageSize         int32    `json:"page_size,omitempty"`
	Search           string   `json:"search,omitempty"`
	Code             string   `json:"code,omitempty"`
	TenantName       string   `json:"tenant_name,omitempty"`
	AppName          string   `json:"app_name,omitempty"`
	Name             string   `json:"name,omitempty"`
	Phone            string   `json:"phone,omitempty"`
	OrgID            int64    `json:"org_id,omitempty"`
	IncludeChildren  bool     `json:"include_children,omitempty"`
	CreatedFrom      int64    `json:"created_from,omitempty"`   // Inclusive Unix seconds; zero is unbounded.
	CreatedBefore    int64    `json:"created_before,omitempty"` // Exclusive Unix seconds; zero is unbounded.
	Sort             string   `json:"sort,omitempty"`
	Desc             bool     `json:"desc,omitempty"`
	AvailableOnly    bool     `json:"available_only,omitempty"`
	TenantAppsOnly   bool     `json:"tenant_apps_only,omitempty"`
	ForAuthorization bool     `json:"for_authorization,omitempty"`
}
type QueryReply struct {
	Total      int64        `json:"total,omitempty"`
	Users      []*User      `json:"users,omitempty"`
	Tenants    []*Tenant    `json:"tenants,omitempty"`
	Members    []*Member    `json:"members,omitempty"`
	Orgs       []*Org       `json:"orgs,omitempty"`
	Positions  []*Position  `json:"positions,omitempty"`
	Apps       []*App       `json:"apps,omitempty"`
	Resources  []*Resource  `json:"resources,omitempty"`
	Operations []*Operation `json:"operations,omitempty"`
	TenantApps []*TenantApp `json:"tenant_apps,omitempty"`
	Roles      []*Role      `json:"roles,omitempty"`
	Districts  []*District  `json:"districts,omitempty"`
	Audits     []*Audit     `json:"audits,omitempty"`
}
type SaveRequest struct {
	Context  *Context  `json:"context,omitempty"`
	Password string    `json:"password,omitempty"`
	User     *User     `json:"user,omitempty"`
	Tenant   *Tenant   `json:"tenant,omitempty"`
	Member   *Member   `json:"member,omitempty"`
	Org      *Org      `json:"org,omitempty"`
	Position *Position `json:"position,omitempty"`
	App      *App      `json:"app,omitempty"`
	Resource *Resource `json:"resource,omitempty"`
	Role     *Role     `json:"role,omitempty"`
}
type SaveReply struct {
	ID int64 `json:"id,omitempty"`
}
type DeleteRequest struct {
	Context        *Context `json:"context,omitempty"`
	Kind           Kind     `json:"kind,omitempty"`
	ID             int64    `json:"id,omitempty"`
	TargetTenantID int64    `json:"target_tenant_id,omitempty"`
}
type StatusRequest struct {
	Context        *Context `json:"context,omitempty"`
	Kind           Kind     `json:"kind,omitempty"`
	ID             int64    `json:"id,omitempty"`
	Status         int32    `json:"status,omitempty"`
	TargetTenantID int64    `json:"target_tenant_id,omitempty"`
}
type PasswordRequest struct {
	Context     *Context `json:"context,omitempty"`
	UserID      int64    `json:"user_id,omitempty"`
	OldPassword string   `json:"old_password,omitempty"`
	NewPassword string   `json:"new_password,omitempty"`
}
type ApproveRequest struct {
	Context       *Context `json:"context,omitempty"`
	TenantID      int64    `json:"tenant_id,omitempty"`
	Approved      bool     `json:"approved,omitempty"`
	AdminPassword string   `json:"admin_password,omitempty"`
}
type AppGrant struct {
	AppID       int64   `json:"app_id,omitempty"`
	ExpiresAt   int64   `json:"expires_at,omitempty"`
	ResourceIDs []int64 `json:"resource_ids,omitempty"`
}
type EntitlementsRequest struct {
	Context   *Context    `json:"context,omitempty"`
	TenantIDs []int64     `json:"tenant_ids,omitempty"`
	Apps      []*AppGrant `json:"apps,omitempty"`
	RevokeIDs []int64     `json:"revoke_ids,omitempty"`
}
type RoleResourcesRequest struct {
	Context  *Context         `json:"context,omitempty"`
	TenantID int64            `json:"tenant_id,omitempty"`
	RoleID   int64            `json:"role_id,omitempty"`
	Grants   []*ResourceGrant `json:"grants,omitempty"`
}
type RoleMembersRequest struct {
	Context  *Context `json:"context,omitempty"`
	TenantID int64    `json:"tenant_id,omitempty"`
	RoleID   int64    `json:"role_id,omitempty"`
	UserIDs  []int64  `json:"user_ids,omitempty"`
	Remove   bool     `json:"remove,omitempty"`
}
type AccessRequest struct {
	Context *Context `json:"context,omitempty"`
	AppCode string   `json:"app_code,omitempty"`
	Method  string   `json:"method,omitempty"`
	Path    string   `json:"path,omitempty"`
}
type AccessReply struct {
	UserID          int64   `json:"user_id,omitempty"`
	TenantID        int64   `json:"tenant_id,omitempty"`
	AppID           int64   `json:"app_id,omitempty"`
	AllWithinTenant bool    `json:"all_within_tenant,omitempty"`
	UserIDs         []int64 `json:"user_ids,omitempty"`
	ServiceKey      string  `json:"service_key,omitempty"`
	RoutePrefix     string  `json:"route_prefix,omitempty"`
}
type UploadRequest struct {
	Context     *Context `json:"context,omitempty"`
	ContentType string   `json:"content_type,omitempty"`
	Size        int64    `json:"size,omitempty"`
	Filename    string   `json:"filename,omitempty"`
	Private     bool     `json:"private,omitempty"`
}
type UploadReply struct {
	ID          int64  `json:"id,omitempty"`
	ObjectKey   string `json:"object_key,omitempty"`
	MaxSize     int64  `json:"max_size,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Private     bool   `json:"private,omitempty"`
}
type ConfirmUploadRequest struct {
	Context *Context `json:"context,omitempty"`
	ID      int64    `json:"id,omitempty"`
}

type FileRequest struct {
	Context   *Context `json:"context,omitempty"`
	ID        int64    `json:"id,omitempty"`
	ObjectKey string   `json:"object_key,omitempty"`
}
