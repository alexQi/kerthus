package core

import (
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/ports"
)

// Writes run under the platform transaction lock. Keep imported duplicates
// editable without creating any new ambiguous email login identities.
func ensureAvailableEmail(u ports.Unit, email string, current identity.User) error {
	if email == "" || (current.ID > 0 && email == current.Email) {
		return nil
	}
	users, err := all[identity.User](u, ports.Users, eq("email", email))
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.ID != current.ID {
			return fault.Conflict("该邮箱已被其他账号使用")
		}
	}
	return nil
}
