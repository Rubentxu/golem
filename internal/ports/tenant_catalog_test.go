package ports

import "testing"

// TestTenantRecord_TierRequired verifies TenantRecord has a tier.
func TestTenantRecord_TierRequired(t *testing.T) {
	t.Parallel()
	record := TenantRecord{
		ID:     "tenant-123",
		CellID: "cell-a",
		Tier:   TenantTierStandard,
		Region: "us-east-1",
	}

	if record.Tier == "" {
		t.Error("expected Tier to be set")
	}
	if record.Tier != TenantTierStandard {
		t.Errorf("expected Tier standard, got %s", record.Tier)
	}

	// Verify regulated tier
	recordRegulated := TenantRecord{
		ID:     "tenant-456",
		CellID: "cell-b",
		Tier:   TenantTierRegulated,
		Region: "eu-west-1",
	}
	if recordRegulated.Tier != TenantTierRegulated {
		t.Errorf("expected Tier regulated, got %s", recordRegulated.Tier)
	}
}
