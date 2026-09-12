package authorization

import (
	pb "kerthus/gen/go/saas/v1"
	"testing"
)

func TestScopeNeverDropsTenantOrEmptyPredicate(t *testing.T) {
	cases := []struct {
		s             *pb.AccessReply
		tenant, owner int64
		want          bool
	}{{nil, 1, 1, false}, {&pb.AccessReply{TenantId: 1, AllWithinTenant: true}, 2, 1, false}, {&pb.AccessReply{TenantId: 1}, 1, 1, false}, {&pb.AccessReply{TenantId: 1, UserIds: []int64{7}}, 1, 7, true}, {&pb.AccessReply{TenantId: 1, UserIds: []int64{7}}, 1, 8, false}, {&pb.AccessReply{TenantId: 1, AllWithinTenant: true}, 1, 8, true}}
	for _, c := range cases {
		if got := AllowsOwner(c.s, c.tenant, c.owner); got != c.want {
			t.Fatalf("scope=%+v tenant=%d owner=%d got %v", c.s, c.tenant, c.owner, got)
		}
	}
}
