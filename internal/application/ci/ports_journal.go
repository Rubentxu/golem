// Package ci provides narrow-port adapters over the graph and journal stores.
package ci

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Rubentxu/golem/internal/application/scm"
	"github.com/Rubentxu/golem/internal/ports"
)

// journalStoreSCMStreamReader implements SCMStreamReader over a JournalStore.
type journalStoreSCMStreamReader struct {
	jrnl ports.JournalStore
}

// NewSCMStreamReaderOverJournal creates an SCMStreamReader that reads from the journal.
func NewSCMStreamReaderOverJournal(jrnl ports.JournalStore) scm.SCMStreamReader {
	return &journalStoreSCMStreamReader{jrnl: jrnl}
}

// CommitObserved implements SCMStreamReader by reading the commit stream from the journal.
func (r *journalStoreSCMStreamReader) CommitObserved(ctx context.Context, tenant, sha string) (*scm.CommitObservedEvent, error) {
	evs, err := r.jrnl.ReadStream(ctx, ports.TenantID(tenant), "commit:"+sha, 0)
	if err != nil {
		return nil, err
	}
	if len(evs) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrCommitNotObserved, sha)
	}

	// Find the EventCommitObserved event in the stream.
	for _, ev := range evs {
		if ev.EventType == "scm.commit.observed.v1" {
			var commit eventCommitObserved
			if err := json.Unmarshal(ev.Payload, &commit); err != nil {
				continue
			}
			return &scm.CommitObservedEvent{
				SHA:        commit.SHA,
				Repository: commit.Repository,
				Message:    commit.Message,
				Author:     commit.Author,
				Timestamp:  commit.Timestamp,
				Implements: commit.Implements,
			}, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrCommitNotObserved, sha)
}

// eventCommitObserved is the raw event stored in the journal.
type eventCommitObserved struct {
	SHA        string   `json:"sha"`
	Repository string   `json:"repository"`
	Message    string   `json:"message"`
	Author     string   `json:"author"`
	Timestamp  int64    `json:"timestamp"`
	Implements []string `json:"implements"`
}
