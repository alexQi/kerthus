package agent

import "testing"

func TestSessionStoreBindsIdentity(t *testing.T) {
	store := NewSessionStore()
	first := store.Create(ContextEnvelope{Trusted: TrustedContext{UserID: 1, TenantID: 2, AppID: 3}})
	if _, err := store.UpdateContext(first.ID, ContextEnvelope{Trusted: TrustedContext{UserID: 9, TenantID: 2, AppID: 3}}); err == nil {
		t.Fatal("expected identity change to be rejected")
	}
	updated, err := store.UpdateContext(first.ID, ContextEnvelope{Trusted: first.Trusted, Page: PageContext{Route: "/dashboard"}})
	if err != nil || updated.Context.Page.Route != "/dashboard" {
		t.Fatalf("context update failed: %+v %v", updated, err)
	}
}
