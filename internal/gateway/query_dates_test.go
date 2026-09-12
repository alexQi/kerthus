package gateway

import (
	"testing"
	"time"
)

func TestTenantCreatedCalendarRangeBecomesHalfOpenUnixSeconds(t *testing.T) {
	zone := time.FixedZone("CST", 8*3600)
	start := time.Date(2026, 9, 11, 0, 0, 0, 0, zone).Unix()
	for _, test := range []struct {
		query        string
		from, before int64
	}{
		{"start_time=2026-09-11&end_time=2026-09-11", start, start + 86400},
		{"start_time=2026-09-11", start, 0},
		{"end_time=2026-09-11", 0, start + 86400},
		{"start_time=&end_time=", 0, 0},
	} {
		m := &mockPlatform{}
		out := request(t, New(m, Config{}), "GET", "/system/tenant/query?"+test.query+"&page=2&pageSize=10", "")
		if out.num("code") != 0 || m.query == nil || m.query.CreatedFrom != test.from || m.query.CreatedBefore != test.before || m.query.Page != 2 {
			t.Fatalf("date range was not mapped correctly: %s", test.query)
		}
	}
	for _, query := range []string{"start_time=invalid", "end_time=2026-02-30", "start_time=2026-09-12&end_time=2026-09-11", "start_time=-1", "start_time=1789056000"} {
		m := &mockPlatform{}
		out := request(t, New(m, Config{}), "GET", "/system/tenant/query?"+query, "")
		if out.num("code") != 400 || m.query != nil {
			t.Fatalf("invalid date silently reached query: %s", query)
		}
	}
}
