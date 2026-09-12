// Package rpcauth authenticates service identities independently of end-user sessions.
package rpcauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"

	"go-micro.dev/v6/metadata"
	"kerthus/internal/saas/domain/fault"
)

const CredentialHeader = "X-Kerthus-Service-Key"

type Config struct {
	GatewayKey string
	AppKeys    map[string]string
}
type Authenticator struct {
	gateway [32]byte
	apps    map[string][32]byte
}

func New(c Config) (*Authenticator, error) {
	if len(c.GatewayKey) < 32 {
		return nil, fmt.Errorf("RPC gateway key must have at least 32 characters")
	}
	a := &Authenticator{gateway: sha256.Sum256([]byte(c.GatewayKey)), apps: map[string][32]byte{}}
	seen := map[[32]byte]bool{a.gateway: true}
	for code, key := range c.AppKeys {
		h := sha256.Sum256([]byte(key))
		if code == "" || len(key) < 32 || seen[h] {
			return nil, fmt.Errorf("invalid or duplicate RPC application credential")
		}
		seen[h] = true
		a.apps[code] = h
	}
	return a, nil
}
func WithCredential(ctx context.Context, key string) context.Context {
	return metadata.Set(ctx, CredentialHeader, key)
}

// Authorize never trusts caller-provided actor, tenant, or application metadata.
func (a *Authenticator) Authorize(ctx context.Context, method, appCode string) error {
	md, _ := metadata.FromContext(ctx)
	key := ""
	for k, v := range md {
		if strings.EqualFold(k, CredentialHeader) {
			if key != "" && key != v {
				return fault.Unauthorized
			}
			key = v
		}
	}
	if key == "" {
		return fault.Unauthorized
	}
	h := sha256.Sum256([]byte(key))
	if subtle.ConstantTimeCompare(h[:], a.gateway[:]) == 1 {
		return nil
	}
	for code, expected := range a.apps {
		if subtle.ConstantTimeCompare(h[:], expected[:]) == 1 {
			if method == "Health" || method == "CheckAccess" && code == appCode {
				return nil
			}
			return fault.Forbidden
		}
	}
	return fault.Unauthorized
}
