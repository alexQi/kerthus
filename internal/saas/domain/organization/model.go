package organization

type Org struct {
	ID        int64
	TenantID  int64
	ParentID  int64
	UnitID    int64
	Type      string
	Name      string
	ShortName string
	Status    int32
	Sort      int32
	Remark    string
	CreatedAt int64
	UpdatedAt int64
}
type Position struct {
	ID        int64
	TenantID  int64
	OrgID     int64
	Name      string
	Status    int32
	Remark    string
	CreatedAt int64
	UpdatedAt int64
}
type MemberOrg struct {
	ID       int64
	TenantID int64
	UserID   int64
	OrgID    int64
}
type MemberPosition struct {
	ID         int64
	TenantID   int64
	UserID     int64
	PositionID int64
}
