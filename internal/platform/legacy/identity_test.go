package legacy

import (
	"testing"

	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/ports"
)

type identityImportUnit struct {
	ports.Unit
	counts map[ports.Table]int64
	users  map[int64]identity.User
}

func (u *identityImportUnit) Insert(table ports.Table, value any) error {
	u.counts[table]++
	if table == ports.Users {
		user := *value.(*identity.User)
		u.users[user.ID] = user
	}
	return nil
}

func (u *identityImportUnit) Count(table ports.Table, _ ports.Filter) (int64, error) {
	return u.counts[table], nil
}

func TestImportRetainsEachAccountLoginPolicy(t *testing.T) {
	// Public synthetic credentials, never a copy of a user's password hash.
	const digest = "22b8a3d1df47ac2f82b46e9f10cfb1c2"
	snapshot := map[string][]Row{
		"d_user": {
			{"id": 1, "phone": "13900000001", "password": digest, "status": 1, "is_signal_login": 1},
			{"id": 2, "phone": "13900000002", "password": digest, "status": 0, "is_signal_login": 0},
		},
		"d_tenant":           {{"id": 1, "status": 1, "verify_status": 1, "address": "[]", "address_code": "[]"}},
		"d_tenant_employee":  {{"id": 1, "tenant_id": 1, "user_id": 1}},
		"d_tenant_role":      {{"id": 1, "tenant_id": 1, "code": "admin", "status": 1}},
		"d_tenant_role_item": {{"id": 1, "tenant_id": 1, "role_id": 1, "item_id": 1, "type": 0}},
	}
	u := &identityImportUnit{counts: map[ports.Table]int64{}, users: map[int64]identity.User{}}
	r := &Report{Imported: map[string]int{}, Excluded: map[string]int{}, ExcludedIDs: map[string][]int64{}}
	if e := importRows(u, snapshot, CatalogResult{}, r); e != nil {
		t.Fatal(e)
	}
	if len(u.users) != 2 || !u.users[1].SingleLogin || u.users[2].SingleLogin || u.users[2].Status != 0 {
		t.Fatal("import lost account-specific login policy or status")
	}
	for _, user := range u.users {
		if user.PasswordHash != "legacy-beehive$"+digest || user.AuthVersion != 1 {
			t.Fatal("import changed credential or initial session version")
		}
	}
}
