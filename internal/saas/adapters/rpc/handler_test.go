package rpc

import (
	"context"
	microerrors "go-micro.dev/v6/errors"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/platform/rpcauth"
	"strings"
	"testing"
)

func TestRejectBeforeUsecase(t *testing.T) {
	key := strings.Repeat("g", 32)
	app := strings.Repeat("a", 32)
	h, e := New(nil, rpcauth.Config{GatewayKey: key, AppKeys: map[string]string{"notes": app}})
	if e != nil {
		t.Fatal(e)
	}
	if e = h.Login(context.Background(), &pb.LoginRequest{}, &pb.LoginReply{}); e == nil || microerrors.FromError(e).Code != 401 {
		t.Fatalf("missing principal: %v", e)
	}
	ctx := rpcauth.WithCredential(context.Background(), app)
	if e = h.Save(ctx, &pb.SaveRequest{}, &pb.SaveReply{}); e == nil || microerrors.FromError(e).Code != 403 {
		t.Fatalf("app mutation: %v", e)
	}
	if e = h.CheckAccess(ctx, &pb.AccessRequest{AppCode: "other"}, &pb.AccessReply{}); e == nil || microerrors.FromError(e).Code != 403 {
		t.Fatalf("cross app: %v", e)
	}
}

func TestFileDownloadAndCleanupRequireGatewayPrincipal(t *testing.T) {
	appKey := strings.Repeat("a", 32)
	h, e := New(nil, rpcauth.Config{GatewayKey: strings.Repeat("g", 32), AppKeys: map[string]string{"notes": appKey}})
	if e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct {
		ctx  context.Context
		want int32
	}{{context.Background(), 401}, {rpcauth.WithCredential(context.Background(), appKey), 403}} {
		for name, call := range map[string]func() error{
			"download": func() error {
				return h.ResolveDownload(test.ctx, &pb.FileRequest{Id: 1, ObjectKey: "private/pending"}, &pb.UploadReply{})
			},
			"cancel": func() error {
				return h.CancelUpload(test.ctx, &pb.FileRequest{Id: 1, ObjectKey: "private/pending"}, &pb.Result{})
			},
		} {
			if e := call(); e == nil || microerrors.FromError(e).Code != test.want {
				t.Fatalf("%s principal: %v", name, e)
			}
		}
	}
}
