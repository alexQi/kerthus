package tenant

type Tenant struct {
	ID               int64
	Name             string
	Logo             string
	ContactPerson    string
	ContactPhone     string
	ContactEmail     string
	CreditCode       string
	AddressJSON      string
	AddressDetail    string
	Description      string
	Status           int32
	VerifyStatus     int32
	ExpiresAt        int64
	BootstrapVersion int64
	CreatedAt        int64
	UpdatedAt        int64
	AgentProvider    string
	AgentModel       string
	AgentEndpoint    string
	AgentAPIKey      string
	AgentEnabled     bool
	AgentProviders   string
}

type Member struct {
	ID            int64
	TenantID      int64
	UserID        int64
	Status        int32
	DefaultAppID  int64
	DefaultUnitID int64
	DefaultOrgID  int64
	IsDefault     bool
	CreatedAt     int64
	UpdatedAt     int64
}
