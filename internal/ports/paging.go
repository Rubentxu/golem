package ports

import "context"

// AlertSeverity ranks alert severity (REQ-SLO-004).
type AlertSeverity string

const (
	AlertSeverityCritical AlertSeverity = "critical"
	AlertSeverityHigh    AlertSeverity = "high"
	AlertSeverityMedium  AlertSeverity = "medium"
	AlertSeverityLow     AlertSeverity = "low"
)

// Alert represents an alert to be sent to operators (REQ-SLO-004).
type Alert struct {
	Severity   AlertSeverity `json:"severity"`
	Route      string        `json:"route"`       // routing destination (e.g., webhook URL, pager duty key)
	Message    string        `json:"message"`
	SLIName   string        `json:"sli_name,omitempty"`
	BudgetLeft float64      `json:"budget_left,omitempty"`
}

// Paging routes alerts to operators (REQ-SLO-004).
type Paging interface {
	// Page sends an alert to operators.
	Page(ctx context.Context, alert Alert) error
}
