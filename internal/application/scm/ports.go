// Package scm hosts the application handlers of the SCM bounded context.
package scm

import "context"

// SCMStreamReader is the narrow port for reading SCM commit streams.
// It replaces the general-purpose JournalStore for SCM-specific reads.
type SCMStreamReader interface {
	// CommitObserved returns the CommitObserved event for a given SHA, if it exists.
	// Returns ErrCommitNotFound if the commit has not been observed.
	CommitObserved(ctx context.Context, tenant, sha string) (*CommitObservedEvent, error)
}

// CommitObservedEvent is the SCM commit observed event.
type CommitObservedEvent struct {
	SHA        string
	Repository string
	Message    string
	Author     string
	Timestamp  int64
	Implements []string // requirement IDs this commit implements
}
