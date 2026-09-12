package identity

type User struct {
	ID            int64
	Phone         string
	Email         string
	Name          string
	Avatar        string
	Sex           int32
	Status        int32
	PasswordHash  string
	PlatformAdmin bool
	SingleLogin   bool
	AuthVersion   int64
	CreatedAt     int64
	UpdatedAt     int64
}

type Session struct {
	UserID      int64 `json:"user_id"`
	AuthVersion int64 `json:"auth_version"`
	ExpiresAt   int64 `json:"expires_at"`
}
