package gateway

import (
	"context"
	"testing"

	"go-micro.dev/v6/client"
	pb "kerthus/gen/go/saas/v1"
)

type timestampPlatform struct{ mockPlatform }

const fixtureCreatedAt int64 = 1700000000
const fixtureUpdatedAt int64 = 1700000010

func (m *timestampPlatform) Profile(context.Context, *pb.Context, ...client.CallOption) (*pb.User, error) {
	return &pb.User{Id: 7, Name: "Alice", Sex: 0, CreatedAt: fixtureCreatedAt, UpdatedAt: fixtureUpdatedAt}, nil
}

func (m *timestampPlatform) Query(ctx context.Context, q *pb.QueryRequest, options ...client.CallOption) (*pb.QueryReply, error) {
	user, _ := m.Profile(ctx, q.Context)
	return &pb.QueryReply{Total: 1,
		Users: []*pb.User{user}, Members: []*pb.Member{{User: user}},
		Positions:  []*pb.Position{{Id: 8, Name: "Position", CreatedAt: fixtureCreatedAt, UpdatedAt: fixtureUpdatedAt}},
		TenantApps: []*pb.TenantApp{{Id: 9, TenantId: 12, AppId: 5, CreatedAt: fixtureCreatedAt, UpdatedAt: fixtureUpdatedAt}},
	}, nil
}

func TestLegacyDTOsExposeReadOnlyTimestampsAndUnsetSex(t *testing.T) {
	h := New(&timestampPlatform{}, Config{})
	for _, path := range []string{"/system/user/query", "/system/employee/query", "/system/employee/info?id=7", "/system/user/profile", "/system/position/query", "/system/app/queryTeantAuthorizes"} {
		out := request(t, h, "GET", path, "")
		if out.num("code") != 0 {
			t.Fatalf("DTO request failed: %s", path)
		}
		row := out.obj("data")
		if items, ok := row["items"].([]any); ok {
			row = object(items[0])
		}
		if row.num("created_at") != fixtureCreatedAt || row.num("updated_at") != fixtureUpdatedAt {
			t.Fatalf("timestamps were dropped: %s", path)
		}
		if path == "/system/position/query" || path == "/system/app/queryTeantAuthorizes" {
			continue
		}
		if sex, present := row["sex"]; !present || sex != float64(0) {
			t.Fatalf("valid unset sex omitted: %s", path)
		}
	}
}
