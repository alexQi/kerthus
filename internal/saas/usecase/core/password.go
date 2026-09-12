package core

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"kerthus/internal/saas/domain/fault"
	"kerthus/internal/saas/domain/identity"
	"kerthus/internal/saas/ports"
)

// LegacyPasswordPrefix explicitly identifies imported Beehive credentials.
// Unmarked MD5 values are never accepted as passwords.
const LegacyPasswordPrefix = "legacy-beehive$"

func verifyPassword(stored, password string) bool {
	if strings.HasPrefix(stored, longPasswordPrefix) {
		return verifyLongPassword(stored, password)
	}
	if !strings.HasPrefix(stored, LegacyPasswordPrefix) {
		// bcrypt must never silently treat a suffix beyond its input limit as
		// irrelevant. Only the explicitly versioned long-password format accepts it.
		return len(password) <= 72 && bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
	}
	digest := strings.TrimPrefix(stored, LegacyPasswordPrefix)
	if len(digest) != 32 || password == "" {
		return false
	}
	for _, ch := range digest {
		if !(ch >= '0' && ch <= '9') && !(ch >= 'a' && ch <= 'f') {
			return false
		}
	}
	sha := sha1.Sum([]byte(password))
	md := md5.Sum([]byte(password))
	combined := hex.EncodeToString(sha[:]) + hex.EncodeToString(md[:])
	actual := md5.Sum([]byte(combined))
	return subtle.ConstantTimeCompare([]byte(digest), []byte(hex.EncodeToString(actual[:]))) == 1
}

// finalizeLogin rechecks the authenticated snapshot under the serialized write
// lock before issuing a session. Password upgrade and single-login revocation
// share this transaction, so a concurrent reset/disable/login cannot be lost.
func (s *Service) finalizeLogin(ctx context.Context, authenticated *identity.User, password string) error {
	var hash []byte
	if strings.HasPrefix(authenticated.PasswordHash, LegacyPasswordPrefix) {
		// Preserve even short legacy credentials while strengthening storage.
		var e error
		hash, e = upgradeLegacyPassword(password)
		if e != nil {
			return e
		}
	}
	return s.DB.Write(ctx, func(u ports.Unit) error {
		var current identity.User
		if e := u.Get(ports.Users, authenticated.ID, &current); e != nil {
			return e
		}
		if current.Status != 1 || current.AuthVersion != authenticated.AuthVersion || current.SingleLogin != authenticated.SingleLogin || subtle.ConstantTimeCompare([]byte(current.PasswordHash), []byte(authenticated.PasswordHash)) != 1 {
			return fault.New(401, "账号状态已变更，请重新登录")
		}
		if len(hash) > 0 {
			current.PasswordHash = string(hash)
		}
		if current.SingleLogin {
			current.AuthVersion++
		}
		if len(hash) > 0 || current.SingleLogin {
			if e := u.Save(ports.Users, &current); e != nil {
				return e
			}
		}
		authenticated.AuthVersion = current.AuthVersion
		return nil
	})
}

// Long imported passwords use standard Argon2id PHC encoding. New passwords and
// short imported credentials keep the existing bcrypt policy. Strict, fixed
// parameters prevent a malformed stored hash from requesting unbounded work.
const (
	longPasswordPrefix             = "$argon2id$"
	longPasswordParameters         = "v=19$m=19456,t=2,p=1$"
	longPasswordMemory      uint32 = 19 * 1024
	longPasswordIterations  uint32 = 2
	longPasswordParallelism uint8  = 1
	longPasswordSaltSize           = 16
	longPasswordKeySize     uint32 = 32
)

func upgradeLegacyPassword(password string) ([]byte, error) {
	if len(password) <= 72 {
		return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	}
	var salt [longPasswordSaltSize]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return nil, err
	}
	key := argon2.IDKey([]byte(password), salt[:], longPasswordIterations, longPasswordMemory, longPasswordParallelism, longPasswordKeySize)
	encoded := longPasswordPrefix + longPasswordParameters + base64.RawStdEncoding.EncodeToString(salt[:]) + "$" + base64.RawStdEncoding.EncodeToString(key)
	return []byte(encoded), nil
}

func verifyLongPassword(stored, password string) bool {
	if password == "" || !strings.HasPrefix(stored, longPasswordPrefix+longPasswordParameters) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(stored, longPasswordPrefix+longPasswordParameters), "$")
	if len(parts) != 2 || len(parts[0]) != base64.RawStdEncoding.EncodedLen(longPasswordSaltSize) || len(parts[1]) != base64.RawStdEncoding.EncodedLen(int(longPasswordKeySize)) {
		return false
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[0])
	if err != nil || len(salt) != longPasswordSaltSize {
		return false
	}
	expected, err := base64.RawStdEncoding.Strict().DecodeString(parts[1])
	if err != nil || len(expected) != int(longPasswordKeySize) {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, longPasswordIterations, longPasswordMemory, longPasswordParallelism, longPasswordKeySize)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
