package agent

type RiskLevel string

const (
	RiskRead       RiskLevel = "read"
	RiskWrite      RiskLevel = "write"
	RiskDelete     RiskLevel = "delete"
	RiskPermission RiskLevel = "permission"
	RiskExport     RiskLevel = "export"
)

type ToolManifest struct {
	ID                   string    `json:"tool_id"`
	Version              string    `json:"version"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	RequiredScopes       []string  `json:"required_scopes"`
	Risk                 RiskLevel `json:"risk_level"`
	RequiresConfirmation bool      `json:"requires_confirmation"`
	TargetService        string    `json:"target_service"`
	TimeoutMilliseconds  int       `json:"timeout_ms"`
	Idempotent           bool      `json:"idempotent"`
}

func (t ToolManifest) NeedsConfirmation() bool {
	return t.RequiresConfirmation || t.Risk != RiskRead
}
