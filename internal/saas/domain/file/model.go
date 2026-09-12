package file

type File struct {
	ID          int64
	TenantID    int64
	AppID       int64
	UserID      int64
	ObjectKey   string
	ContentType string
	Filename    string
	Private     bool
	Size        int64
	Status      int32
	CreatedAt   int64
}
