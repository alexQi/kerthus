package access

import "sort"

type Role struct {
	ID            int64
	TenantID      int64
	Code          string
	Name          string
	Remark        string
	Status        int32
	Administrator bool
	CreatedAt     int64
	UpdatedAt     int64
}
type RoleMember struct {
	ID       int64
	TenantID int64
	RoleID   int64
	UserID   int64
}
type Grant struct {
	ID         int64
	TenantID   int64
	RoleID     int64
	AppID      int64
	ResourceID int64
	DataScope  int32
}

// Scope keeps the empty set distinct from all rows within a tenant.
type Scope struct {
	All     bool
	UserIDs []int64
}

func Union(scopes ...Scope) Scope {
	out := Scope{}
	seen := map[int64]bool{}
	for _, s := range scopes {
		if s.All {
			return Scope{All: true}
		}
		for _, id := range s.UserIDs {
			seen[id] = true
		}
	}
	for id := range seen {
		out.UserIDs = append(out.UserIDs, id)
	}
	sort.Slice(out.UserIDs, func(i, j int) bool { return out.UserIDs[i] < out.UserIDs[j] })
	return out
}
