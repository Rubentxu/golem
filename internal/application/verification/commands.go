// Package verification hosts the application handlers of the Test
// context.
package verification

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/ports"
	domainver "github.com/Rubentxu/golem/internal/verification"
)

// EntityRefReader is the kernel-level narrow port for entity existence checks.
type EntityRefReader = ports.EntityRefReader

var (
	ErrEmptyCase        = errors.New("verification: test case is mandatory")
	ErrInvalidRunStatus = errors.New("verification: status must be passed|failed|skipped")
	ErrUnknownTarget    = errors.New("verification: verified entity not found")
)

// Command names of this context.
const (
	CmdReportTestRun = "verification.report-test-run"
)

// ReportTestRun is the payload of CmdReportTestRun. RunID may be empty
// (server-generated); providers with stable external ids pass their own
// and rely on command dedup for idempotency.
type ReportTestRun struct {
	RunID    string `json:"run_id,omitempty"`
	Case     string `json:"case"`
	Status   string `json:"status"`
	Verifies string `json:"verifies"`
}

// ReportTestRunHandler returns the handler for CmdReportTestRun. The
// verified entity must exist in the graph projection (point read) —
// same consistency model as work links.
func ReportTestRunHandler(gen ports.IDGenerator, entityReader EntityRefReader) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(ReportTestRun)
		if !ok {
			return nil, errors.New("verification: payload must be verification.ReportTestRun")
		}
		if strings.TrimSpace(p.Case) == "" {
			return nil, ErrEmptyCase
		}
		switch strings.ToLower(strings.TrimSpace(p.Status)) {
		case "passed", "failed", "skipped":
		default:
			return nil, fmt.Errorf("%w: %q", ErrInvalidRunStatus, p.Status)
		}
		target := strings.TrimSpace(p.Verifies)
		if target == "" {
			return nil, fmt.Errorf("%w: empty verifies", ErrUnknownTarget)
		}
		// Verify the target requirement exists via EntityRefReader.
		exists, err := entityReader.Exists(ctx, cmd.TenantID, "Requirement", target)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrUnknownTarget, target)
		}
		if !exists {
			return nil, fmt.Errorf("%w: %s", ErrUnknownTarget, target)
		}

		runID := strings.TrimSpace(p.RunID)
		if runID == "" {
			runID = gen.NewID()
		}
		return []appcmd.EventDraft{{
			EventType:     domainver.EventTestRunReported,
			StreamID:      "testrun:" + runID,
			SchemaVersion: 1,
			Payload: domainver.TestRunReported{
				RunID: runID, TestCase: strings.TrimSpace(p.Case),
				Status: strings.ToLower(strings.TrimSpace(p.Status)), Verifies: target,
			},
		}}, nil
	}
}
