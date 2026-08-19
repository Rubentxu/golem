package ports

import (
	"testing"
)

func TestCommandRecordZeroValue(t *testing.T) {
	var rec CommandRecord
	if rec.CommandID != "" {
		t.Errorf("CommandRecord zero value: CommandID = %q, want empty", rec.CommandID)
	}
	if rec.CommandKind != "" {
		t.Errorf("CommandRecord zero value: CommandKind = %q, want empty", rec.CommandKind)
	}
	if rec.TenantID != "" {
		t.Errorf("CommandRecord zero value: TenantID = %q, want empty", rec.TenantID)
	}
	if rec.Actor.Type != "" || rec.Actor.ID != "" {
		t.Errorf("CommandRecord zero value: Actor = %+v, want zero Actor", rec.Actor)
	}
	if rec.CorrelationID != "" {
		t.Errorf("CommandRecord zero value: CorrelationID = %q, want empty", rec.CorrelationID)
	}
	if rec.Fingerprint != "" {
		t.Errorf("CommandRecord zero value: Fingerprint = %q, want empty", rec.Fingerprint)
	}
}

func TestCommandJournalReceiptZeroValue(t *testing.T) {
	var receipt CommandJournalReceipt
	if receipt.CommandID != "" {
		t.Errorf("CommandJournalReceipt zero value: CommandID = %q, want empty", receipt.CommandID)
	}
	if len(receipt.EventIDs) != 0 {
		t.Errorf("CommandJournalReceipt zero value: EventIDs len = %d, want 0", len(receipt.EventIDs))
	}
	if receipt.Position != 0 {
		t.Errorf("CommandJournalReceipt zero value: Position = %d, want 0", receipt.Position)
	}
	if receipt.Tenant != "" {
		t.Errorf("CommandJournalReceipt zero value: Tenant = %q, want empty", receipt.Tenant)
	}
	if receipt.Actor.Type != "" || receipt.Actor.ID != "" {
		t.Errorf("CommandJournalReceipt zero value: Actor = %+v, want zero Actor", receipt.Actor)
	}
	if receipt.Correlation != "" {
		t.Errorf("CommandJournalReceipt zero value: Correlation = %q, want empty", receipt.Correlation)
	}
	if receipt.Duplicate {
		t.Errorf("CommandJournalReceipt zero value: Duplicate = true, want false")
	}
}
