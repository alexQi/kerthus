package gateway

import (
	"time"

	pb "kerthus/gen/go/saas/v1"
	"kerthus/internal/saas/domain/fault"
)

// The console sends calendar dates. Include both selected dates in China time,
// represented internally as [start midnight, midnight after the final day).
func tenantCreatedRange(q *pb.QueryRequest, p payload) error {
	for _, key := range []string{"start_time", "end_time"} {
		value := p.str(key)
		if value == "" {
			continue
		}
		date, err := time.ParseInLocation("2006-01-02", value, time.FixedZone("CST", 8*3600))
		if err != nil || date.Unix() <= 0 {
			return fault.Invalid("创建时间请使用 YYYY-MM-DD 日期")
		}
		if key == "start_time" {
			q.CreatedFrom = date.Unix()
		} else {
			q.CreatedBefore = date.AddDate(0, 0, 1).Unix()
		}
	}
	if q.CreatedFrom > 0 && q.CreatedBefore > 0 && q.CreatedFrom >= q.CreatedBefore {
		return fault.Invalid("创建时间范围不合法")
	}
	return nil
}
