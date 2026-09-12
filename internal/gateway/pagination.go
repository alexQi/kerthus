package gateway

import (
	"context"
	"google.golang.org/protobuf/proto"
	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/saas/domain/fault"
)

// Full-replacement editors must load every page before allowing a save.
func (g *Gateway) queryAll(ctx context.Context, in *pb.QueryRequest) (*pb.QueryReply, error) {
	q := proto.Clone(in).(*pb.QueryRequest)
	q.Page = 1
	q.PageSize = 1000
	if q.Sort == "" {
		q.Sort = "id"
		if q.Kind == pb.Kind_ORGS || q.Kind == pb.Kind_RESOURCES {
			q.Sort = "sort"
		}
	}
	out := &pb.QueryReply{}
	for {
		r, e := g.client.Query(ctx, q)
		if e != nil {
			return nil, e
		}
		out.Total = r.Total
		out.Users = append(out.Users, r.Users...)
		out.Tenants = append(out.Tenants, r.Tenants...)
		out.Members = append(out.Members, r.Members...)
		out.Orgs = append(out.Orgs, r.Orgs...)
		out.Positions = append(out.Positions, r.Positions...)
		out.Apps = append(out.Apps, r.Apps...)
		out.Resources = append(out.Resources, r.Resources...)
		out.Operations = append(out.Operations, r.Operations...)
		out.TenantApps = append(out.TenantApps, r.TenantApps...)
		out.Roles = append(out.Roles, r.Roles...)
		out.Districts = append(out.Districts, r.Districts...)
		out.Audits = append(out.Audits, r.Audits...)
		count := func(r *pb.QueryReply) int {
			switch q.Kind {
			case pb.Kind_USERS:
				return len(r.Users)
			case pb.Kind_TENANTS:
				return len(r.Tenants)
			case pb.Kind_MEMBERS:
				return len(r.Members)
			case pb.Kind_ORGS:
				return len(r.Orgs)
			case pb.Kind_POSITIONS:
				return len(r.Positions)
			case pb.Kind_APPS:
				return len(r.Apps)
			case pb.Kind_RESOURCES:
				return len(r.Resources)
			case pb.Kind_OPERATIONS:
				return len(r.Operations)
			case pb.Kind_TENANT_APPS:
				return len(r.TenantApps)
			case pb.Kind_ROLES:
				return len(r.Roles)
			case pb.Kind_DISTRICTS:
				return len(r.Districts)
			case pb.Kind_AUDITS:
				return len(r.Audits)
			}
			return 0
		}
		if int64(count(out)) >= out.Total {
			return out, nil
		}
		if count(r) == 0 || q.Page >= 100 {
			return nil, fault.Conflict("完整数据未加载，请缩小查询范围后重试")
		}
		q.Page++
	}
}
