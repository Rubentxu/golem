// Package scm hosts the application handlers of the SCM context.
package scm

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	domainscm "github.com/Rubentxu/golem/internal/scm"
)

var (
	ErrEmptySHA   = errors.New("scm: commit sha is mandatory")
	ErrInvalidSHA = errors.New("scm: commit sha must be 40 hex (git) or 64 hex (sha256)")
	ErrEmptyRepo  = errors.New("scm: repository is mandatory")
)

// shaPattern accepts full git (40) and sha256 (64) hex ids.
var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// Command names of this context.
const (
	CmdObserveCommit = "scm.observe-commit"
)

// ObserveCommit is the payload of CmdObserveCommit.
type ObserveCommit struct {
	SHA        string   `json:"sha"`
	Repository string   `json:"repository"`
	Message    string   `json:"message"`
	Implements []string `json:"implements"`
}

// ObserveCommitHandler returns the handler for CmdObserveCommit.
// Re-observing the same sha is idempotent at the journal level (same
// stream, dedup by command id from providers).
func ObserveCommitHandler() appcmd.Handler {
	return func(_ context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(ObserveCommit)
		if !ok {
			return nil, errors.New("scm: payload must be scm.ObserveCommit")
		}
		sha := strings.ToLower(strings.TrimSpace(p.SHA))
		if sha == "" {
			return nil, ErrEmptySHA
		}
		if !shaPattern.MatchString(sha) {
			return nil, fmt.Errorf("%w: got %d chars", ErrInvalidSHA, len(sha))
		}
		if strings.TrimSpace(p.Repository) == "" {
			return nil, ErrEmptyRepo
		}
		impl := make([]string, 0, len(p.Implements))
		for _, r := range p.Implements {
			if id := strings.TrimSpace(r); id != "" {
				impl = append(impl, id)
			}
		}

		return []appcmd.EventDraft{{
			EventType:     domainscm.EventCommitObserved,
			StreamID:      "commit:" + sha,
			SchemaVersion: 1,
			Payload: domainscm.CommitObserved{
				SHA: sha, Repository: strings.TrimSpace(p.Repository),
				Message: strings.TrimSpace(p.Message), Implements: impl,
			},
		}}, nil
	}
}
