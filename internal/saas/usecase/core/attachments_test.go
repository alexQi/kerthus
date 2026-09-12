package core

import (
	"context"
	"strings"
	"testing"

	"kerthus/internal/saas/domain/catalog"
	file "kerthus/internal/saas/domain/file"
	"kerthus/internal/saas/ports"
	c "kerthus/internal/saas/usecase/contracts"
)

type attachmentObjects struct {
	size    int64
	ct      string
	removed []string
}

func (s *attachmentObjects) Stat(context.Context, string) (int64, string, error) {
	return s.size, s.ct, nil
}
func (s *attachmentObjects) Remove(_ context.Context, key string) error {
	s.removed = append(s.removed, key)
	return nil
}

func TestMySQLAttachmentsAreScopedAndRespectCurrentAccess(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	objects := &attachmentObjects{size: 11, ct: "application/pdf"}
	f.svc.Objects = objects
	pending, err := f.svc.CreateUpload(ctx, &c.UploadRequest{Context: f.ac, Filename: "../../财务报告.pdf", ContentType: objects.ct, Size: objects.size, Private: true})
	if err != nil {
		t.Fatal(err)
	}
	if !pending.Private || pending.Filename != "财务报告.pdf" || !strings.HasPrefix(pending.ObjectKey, "private/") || pending.MaxSize != 50<<20 {
		t.Fatalf("private metadata=%+v", pending)
	}
	_, err = f.svc.ResolveDownload(ctx, &c.FileRequest{Context: f.ac, ObjectKey: pending.ObjectKey})
	wantCode(t, err, 404)
	_, err = f.svc.ConfirmUpload(ctx, &c.ConfirmUploadRequest{Context: f.sc, ID: pending.ID})
	wantCode(t, err, 403)
	confirmed, err := f.svc.ConfirmUpload(ctx, &c.ConfirmUploadRequest{Context: f.ac, ID: pending.ID})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Size != 11 || confirmed.ContentType != "application/pdf" {
		t.Fatal("confirmed metadata missing")
	}
	for _, actor := range []*c.Context{f.ac, f.sc} {
		got, e := f.svc.ResolveDownload(ctx, &c.FileRequest{Context: actor, ObjectKey: pending.ObjectKey})
		if e != nil || got.ID != pending.ID {
			t.Fatalf("same-tenant access: %+v %v", got, e)
		}
	}
	for _, actor := range []*c.Context{f.bc, f.sharedB} {
		_, e := f.svc.ResolveDownload(ctx, &c.FileRequest{Context: actor, ObjectKey: pending.ObjectKey})
		wantCode(t, e, 404)
	}
	_, err = f.svc.ResolveDownload(ctx, &c.FileRequest{Context: f.ac, ID: pending.ID, ObjectKey: "another"})
	wantCode(t, err, 404)
	_, err = f.svc.CancelUpload(ctx, &c.FileRequest{Context: f.ac, ID: pending.ID, ObjectKey: pending.ObjectKey})
	wantCode(t, err, 409)
	// Grant a different valid application to demonstrate the key cannot change app scope.
	f.write(func(u ports.Unit) error {
		r := catalog.Resource{AppID: f.extension.ID, Code: "notes:read", Name: "Notes", Type: "view", Status: 1, MetaJSON: "{}"}
		if e := u.Save(ports.Resources, &r); e != nil {
			return e
		}
		return f.svc.grantApplication(u, f.ta.ID, &c.AppGrant{AppID: f.extension.ID, ResourceIDs: []int64{r.ID}})
	})
	otherApp := *f.ac
	otherApp.AppID = f.extension.ID
	_, err = f.svc.ResolveDownload(ctx, &c.FileRequest{Context: &otherApp, ObjectKey: pending.ObjectKey})
	wantCode(t, err, 404)
	// Revocation is evaluated on every download and upload, never frozen at upload time.
	f.write(func(u ports.Unit) error { f.viewerA.Status = 0; return u.Save(ports.Roles, &f.viewerA) })
	_, err = f.svc.ResolveDownload(ctx, &c.FileRequest{Context: f.sc, ObjectKey: pending.ObjectKey})
	wantCode(t, err, 403)
	_, err = f.svc.CreateUpload(ctx, &c.UploadRequest{Context: f.sc, Filename: "a.pdf", ContentType: objects.ct, Size: 11, Private: true})
	wantCode(t, err, 403)
	f.write(func(u ports.Unit) error { f.app.Status = 0; return u.Save(ports.Apps, &f.app) })
	_, err = f.svc.ResolveDownload(ctx, &c.FileRequest{Context: f.ac, ObjectKey: pending.ObjectKey})
	wantCode(t, err, 403)
	f.write(func(u ports.Unit) error {
		f.app.Status = 1
		if e := u.Save(ports.Apps, &f.app); e != nil {
			return e
		}
		ent, e := first[catalog.Entitlement](u, ports.Entitlements, filter("tenant_id", f.ta.ID, "app_id", f.app.ID))
		if e != nil {
			return e
		}
		ent.ExpiresAt = f.svc.now() - 1
		return u.Save(ports.Entitlements, &ent)
	})
	_, err = f.svc.ResolveDownload(ctx, &c.FileRequest{Context: f.ac, ObjectKey: pending.ObjectKey})
	wantCode(t, err, 403)
	if len(objects.removed) != 0 {
		t.Fatal("authorization failure removed confirmed data")
	}
}

func TestMySQLAttachmentCleanupAndImageCompatibility(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	objects := &attachmentObjects{size: 4, ct: "text/plain"}
	f.svc.Objects = objects
	f.svc.AttachmentMaxBytes = 100
	_, err := f.svc.CreateUpload(ctx, &c.UploadRequest{Context: f.ac, ContentType: objects.ct, Size: 101, Private: true})
	wantCode(t, err, 400)
	_, err = f.svc.CreateUpload(ctx, &c.UploadRequest{Context: f.ac, ContentType: objects.ct, Size: 4})
	wantCode(t, err, 400)
	pending, err := f.svc.CreateUpload(ctx, &c.UploadRequest{Context: f.ac, Filename: "fixture.txt", ContentType: objects.ct, Size: 4, Private: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.CancelUpload(ctx, &c.FileRequest{Context: f.bc, ID: pending.ID, ObjectKey: pending.ObjectKey})
	wantCode(t, err, 404)
	if len(objects.removed) != 0 {
		t.Fatal("foreign member removed pending file")
	}
	objects.size = 5
	_, err = f.svc.ConfirmUpload(ctx, &c.ConfirmUploadRequest{Context: f.ac, ID: pending.ID})
	wantCode(t, err, 400)
	var record file.File
	f.read(func(u ports.Unit) error { return u.Get(ports.Files, pending.ID, &record) })
	if record.Status != 0 {
		t.Fatal("mismatched object was published")
	}
	// Cleanup needs the pending capability, not merely a guessed sequential ID.
	_, err = f.svc.CancelUpload(ctx, &c.FileRequest{Context: f.ac, ID: pending.ID, ObjectKey: "wrong-key"})
	wantCode(t, err, 404)
	// Cleanup still works if entitlement is disabled and session disappears mid-transfer.
	f.svc.Sessions.Delete(ctx, f.ac.Token)

	f.write(func(u ports.Unit) error { f.app.Status = 0; return u.Save(ports.Apps, &f.app) })
	_, err = f.svc.CancelUpload(ctx, &c.FileRequest{Context: f.ac, ID: pending.ID, ObjectKey: pending.ObjectKey})
	if err != nil {
		t.Fatal(err)
	}
	if len(objects.removed) != 1 || objects.removed[0] != pending.ObjectKey || f.count(ports.Files, filter("id", pending.ID)) != 0 {
		t.Fatal("failed transfer cleanup did not remove row and object")
	}
	f.write(func(u ports.Unit) error { f.app.Status = 1; return u.Save(ports.Apps, &f.app) })
	f.ac = f.session(f.a, f.ta)
	// The independent 10MB image limit and existing public key format are unchanged.
	image, err := f.svc.CreateUpload(ctx, &c.UploadRequest{Context: f.ac, ContentType: "image/png", Size: 101})
	if err != nil {
		t.Fatal(err)
	}
	if image.Private || image.MaxSize != 10<<20 || !strings.HasPrefix(image.ObjectKey, "public/") || !strings.HasSuffix(image.ObjectKey, ".png") {
		t.Fatalf("image compatibility: %+v", image)
	}
	_, err = f.svc.CreateUpload(ctx, &c.UploadRequest{Context: f.ac, ContentType: "image/png", Size: (10 << 20) + 1})
	wantCode(t, err, 400)
}
