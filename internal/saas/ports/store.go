package ports

import (
	"context"
	"kerthus/internal/saas/domain/identity"
	"time"
)

// Table is a closed set; neither SQL identifiers nor predicates come from HTTP.
type Table string

const (
	Users           Table = "users"
	Tenants         Table = "tenants"
	Members         Table = "members"
	Orgs            Table = "orgs"
	Positions       Table = "positions"
	MemberOrgs      Table = "member_orgs"
	MemberPositions Table = "member_positions"
	Apps            Table = "apps"
	Resources       Table = "resources"
	Operations      Table = "operations"
	Entitlements    Table = "entitlements"
	TenantResources Table = "tenant_resources"
	Roles           Table = "roles"
	RoleMembers     Table = "role_members"
	Grants          Table = "grants"
	Audits          Table = "audits"
	Files           Table = "files"
	Districts       Table = "districts"
)

// Filter values are parameterized. Every column is checked by the adapter.
type Filter struct {
	Equal        map[string]any
	In           map[string][]int64
	Order        string
	Desc         bool
	Page         int
	PageSize     int
	Search       string
	Contains     map[string]string
	GreaterEqual map[string]int64
	LessThan     map[string]int64
}

// Unit is transaction-bound. Models are domain types, never transport DTOs.
// A single storage port avoids duplicating CRUD plumbing across small domains;
// usecases still own typed commands, tenant scoping, invariants and authorization.
type Unit interface {
	Get(Table, int64, any) error
	Find(Table, Filter, any) error
	Count(Table, Filter) (int64, error)
	Save(Table, any) error
	Insert(Table, any) error
	Delete(Table, Filter) error
	LockTenant(int64) error
	LockCatalog() error
}
type Database interface {
	Read(context.Context, func(Unit) error) error
	Write(context.Context, func(Unit) error) error
	Ping(context.Context) error
}
type Sessions interface {
	Put(context.Context, string, identity.Session, time.Duration) error
	Get(context.Context, string) (identity.Session, error)
	Delete(context.Context, string) error
	Ping(context.Context) error
}
type Objects interface {
	Stat(context.Context, string) (int64, string, error)
	Remove(context.Context, string) error
}
