package audit

type Event struct {
	ID        int64
	ActorID   int64
	TenantID  int64
	AppID     int64
	Action    string
	TargetID  int64
	RequestID string
	CreatedAt int64
}
