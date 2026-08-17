// Package scm defines the SCM bounded context: repositories, commits
// and reviews (BOUNDED_CONTEXTS). Commits are observed from providers —
// never authored here — and carry ExternalIdentity (GRAPH_MODEL).
package scm

// CommitObserved is the payload of scm.commit.observed.v1. SHA is the
// immutable commit identity (node id); Implements lists requirement ids
// the commit satisfies (Requirement→Work/Commit trace).
type CommitObserved struct {
	SHA        string   `json:"sha"`
	Repository string   `json:"repository"`
	Message    string   `json:"message"`
	Implements []string `json:"implements,omitempty"`
}

// Event type names of this context.
const (
	EventCommitObserved = "scm.commit.observed.v1"
)
