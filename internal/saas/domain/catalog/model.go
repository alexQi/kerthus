package catalog

type App struct {
	Type            string
	URL             string
	IsPublic        bool
	ID              int64
	Code            string
	Name            string
	Icon            string
	Description     string
	Remark          string
	Version         string
	Status          int32
	ServiceKey      string
	RoutePrefix     string
	Home            string
	FrontendEntry   string
	ManifestHash    string
	ResourceVersion int64
	Managed         bool
	CreatedAt       int64
	UpdatedAt       int64
}
type Resource struct {
	ID           int64
	AppID        int64
	ParentID     int64
	Code         string
	Name         string
	Type         string
	Path         string
	Component    string
	Redirect     string
	OpenWith     string
	Remark       string
	Icon         string
	MetaJSON     string
	Status       int32
	Sort         int32
	IsPublic     bool
	IsDataAccess bool
	CreatedAt    int64
	UpdatedAt    int64
}
type Operation struct {
	ID          int64
	AppID       int64
	ResourceID  int64
	OperationID string
	Method      string
	Path        string
	GroupName   string
	Action      string
}
type Entitlement struct {
	ID        int64
	TenantID  int64
	AppID     int64
	Status    int32
	ExpiresAt int64
	CreatedAt int64
	UpdatedAt int64
}
type TenantResource struct {
	ID         int64
	TenantID   int64
	AppID      int64
	ResourceID int64
}
