// Package authorization is the application-side platform client. Applications
// must call Check before using a tenant ID or scope to query their own data.
package authorization

import (
	"context"
	"fmt"
	"go-micro.dev/v6/metadata"
	pb "kerthus/gen/go/saas/v1"
	"net/http"
	"strconv"
)

type Client struct {
	platform            pb.PlatformService
	appCode, serviceKey string
}

func New(platform pb.PlatformService, appCode, serviceKey string) (*Client, error) {
	if platform == nil || appCode == "" || len(serviceKey) < 32 {
		return nil, fmt.Errorf("platform, bound application code and service credential are required")
	}
	return &Client{platform, appCode, serviceKey}, nil
}
func (c *Client) Check(ctx context.Context, input *pb.Context, method, path string) (*pb.AccessReply, error) {
	ctx = metadata.Set(ctx, "X-Kerthus-Service-Key", c.serviceKey)
	return c.platform.CheckAccess(ctx, &pb.AccessRequest{Context: input, AppCode: c.appCode, Method: method, Path: path})
}
func (c *Client) CheckRequest(r *http.Request) (*pb.AccessReply, error) {
	id := func(k string) int64 { v, _ := strconv.ParseInt(r.Header.Get(k), 10, 64); return v }
	input := &pb.Context{Token: r.Header.Get("access-token"), TenantId: id("tenant-id"), AppId: id("app-id"), UnitId: id("unit-id"), SectionId: id("section-id"), RequestId: r.Header.Get("X-Request-Id")}
	return c.Check(r.Context(), input, r.Method, r.URL.Path)
}

// AllowsOwner never treats an empty scope as unrestricted. The tenant predicate
// remains mandatory even for AllWithinTenant (including platform operators).
func AllowsOwner(scope *pb.AccessReply, tenantID, ownerID int64) bool {
	if scope == nil || scope.TenantId != tenantID {
		return false
	}
	if scope.AllWithinTenant {
		return true
	}
	for _, id := range scope.UserIds {
		if id == ownerID {
			return true
		}
	}
	return false
}
