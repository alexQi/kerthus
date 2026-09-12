package core

import (
	"context"
	"fmt"
	"testing"

	c "kerthus/internal/saas/usecase/contracts"
)

func TestMySQLAuditIdentifiesEntityAfterDeleteAndKeepsFailedWritesOut(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	actor := *f.ac
	actor.RequestID = "audit-acceptance-request"
	org, err := f.svc.Save(ctx, &c.SaveRequest{Context: &actor, Org: &c.Org{Name: "Audit organization", Type: "unit"}})
	if err != nil {
		t.Fatal(err)
	}
	position, err := f.svc.Save(ctx, &c.SaveRequest{Context: &actor, Position: &c.Position{Name: "Audit position", OrgID: org.ID}})
	if err != nil {
		t.Fatal(err)
	}
	role, err := f.svc.Save(ctx, &c.SaveRequest{Context: &actor, Role: &c.Role{Name: "Audit role"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.Save(ctx, &c.SaveRequest{Context: &actor, Role: &c.Role{ID: role.ID, Name: "Audit role edited"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.SetStatus(ctx, &c.StatusRequest{Context: &actor, Kind: c.KindRoles, ID: role.ID, Status: 0}); err != nil {
		t.Fatal(err)
	}
	// Referenced organization deletion fails and must not look like a success.
	_, err = f.svc.Delete(ctx, &c.DeleteRequest{Context: &actor, Kind: c.KindOrgs, ID: org.ID})
	wantCode(t, err, 409)
	for _, item := range []struct {
		kind c.Kind
		id   int64
	}{{c.KindRoles, role.ID}, {c.KindPositions, position.ID}, {c.KindOrgs, org.ID}} {
		if _, err = f.svc.Delete(ctx, &c.DeleteRequest{Context: &actor, Kind: item.kind, ID: item.id}); err != nil {
			t.Fatal(err)
		}
	}
	out, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.ac, Kind: c.KindAudits})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		fmt.Sprintf("organization.save/%d", org.ID):    1,
		fmt.Sprintf("position.save/%d", position.ID):   1,
		fmt.Sprintf("role.save/%d", role.ID):           2,
		fmt.Sprintf("role.disable/%d", role.ID):        1,
		fmt.Sprintf("role.delete/%d", role.ID):         1,
		fmt.Sprintf("position.delete/%d", position.ID): 1,
		fmt.Sprintf("organization.delete/%d", org.ID):  1,
	}
	if out.Total != 8 || len(out.Audits) != 8 {
		t.Fatalf("audit count includes failed write or misses success: %d/%d", out.Total, len(out.Audits))
	}
	for _, event := range out.Audits {
		if event.TargetID <= 0 || event.ActorID != f.a.ID || event.TenantID != f.ta.ID || event.CreatedAt <= 0 || event.RequestID != actor.RequestID {
			t.Fatalf("audit lost provenance: %+v", event)
		}
		key := fmt.Sprintf("%s/%d", event.Action, event.TargetID)
		if want[key] <= 0 {
			t.Fatalf("ambiguous or unexpected audit target: %s", key)
		}
		want[key]--
	}
	for key, count := range want {
		if count != 0 {
			t.Fatalf("missing audit %s", key)
		}
	}
	other, err := f.svc.Query(ctx, &c.QueryRequest{Context: f.bc, Kind: c.KindAudits})
	if err != nil || other.Total != 0 {
		t.Fatal("another tenant can see audit events", err)
	}
}
