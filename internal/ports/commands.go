package ports

import (
	"context"
	"errors"
)

// ErrDuplicateCommand is returned by CommandRegistry.Save when the command
// was already registered (e.g. concurrent duplicate submission).
var ErrDuplicateCommand = errors.New("ports: command already registered")

// CommandReceipt acknowledges an accepted command. Position is the journal
// position of its last event: clients may use it as a replay checkpoint
// for optional read-your-write semantics (ARCHITECTURE — write path).
type CommandReceipt struct {
	CommandID string
	TenantID  TenantID
	EventIDs  []string
	Position  StreamPosition
	// Duplicate is true when the same command_id was already processed:
	// the stored receipt is returned and no new events are journaled
	// (ADR-020 idempotent acceptance).
	Duplicate bool
}

// CommandRegistry deduplicates command processing by command_id. It is the
// command-side inbox of ADR-020. Save must be atomic: concurrent saves of
// the same command_id — within a deployment or across them — yield exactly
// one success and ErrDuplicateCommand for the rest.
type CommandRegistry interface {
	// Find returns the stored receipt for a command; found is false when
	// the command is unknown.
	Find(ctx context.Context, commandID string) (receipt CommandReceipt, found bool, err error)
	// Save registers the receipt of a processed command.
	Save(ctx context.Context, receipt CommandReceipt) error
}
