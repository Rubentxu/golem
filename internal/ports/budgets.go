package ports

// Budget constrains resource usage per execution (ADR-069).
// Stub type to be fully implemented in T14.
type Budget struct {
	TokenCost       float64
	WallClockMs     int
	ToolCalls       int
	ProposalsPerRun int
}

// Validate checks if the budget values are valid.
func (b Budget) Validate() error {
	return nil
}
