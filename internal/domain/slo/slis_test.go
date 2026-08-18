package slo

import (
	"testing"
)

func TestSLI_AllThirteenDefined(t *testing.T) {
	t.Parallel()

	slis := AllSLIs()
	if len(slis) != 13 {
		t.Fatalf("expected 13 SLIs, got %d", len(slis))
	}

	expected := map[SLIName]bool{
		SLICommandLatencyP99:          false,
		SLICommandErrorRate:           false,
		SLISystemAvailability:         false,
		SLIJournalReplayTimeP99:       false,
		SLIAgentEvalPassRate:          false,
		SLIOIDCVerifyLatencyP99:       false,
		SLIOpsConsoleActionLatencyP99: false,
		SLIAuditExportSuccessRate:     false,
		SLIMeteringRollupSuccessRate:  false,
		SLIQuotaCheckLatencyP99:       false,
		SLICellMigrateDurationP99:     false,
		SLISnapshotDurationP99:        false,
		SLIMeterQueryLatencyP99:       false,
	}

	for _, sli := range slis {
		if _, ok := expected[sli.Name]; ok {
			expected[sli.Name] = true
		} else {
			t.Errorf("unexpected SLI: %s", sli.Name)
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("SLI not found: %s", name)
		}
	}
}

func TestGetSLI(t *testing.T) {
	t.Parallel()

	sli, err := GetSLI(SLICommandLatencyP99)
	if err != nil {
		t.Fatalf("GetSLI failed: %v", err)
	}
	if sli.Target != 0.999 {
		t.Errorf("expected target 0.999, got %f", sli.Target)
	}
	if sli.Unit != "ms" {
		t.Errorf("expected unit ms, got %s", sli.Unit)
	}
}

func TestGetSLI_NotFound(t *testing.T) {
	t.Parallel()

	_, err := GetSLI("nonexistent.sli")
	if err == nil {
		t.Fatal("expected error for nonexistent SLI")
	}
}

func TestValidateValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sliName SLIName
		value   float64
		wantErr bool
	}{
		{
			name:    "valid latency value",
			sliName: SLICommandLatencyP99,
			value:   150.0,
			wantErr: false,
		},
		{
			name:    "negative latency",
			sliName: SLICommandLatencyP99,
			value:   -10.0,
			wantErr: true,
		},
		{
			name:    "latency exceeds max",
			sliName: SLICommandLatencyP99,
			value:   70000.0,
			wantErr: true,
		},
		{
			name:    "valid percent",
			sliName: SLISystemAvailability,
			value:   0.999,
			wantErr: false,
		},
		{
			name:    "percent exceeds 100",
			sliName: SLISystemAvailability,
			value:   101.0,
			wantErr: true,
		},
		{
			name:    "unknown SLI",
			sliName: "unknown.sli",
			value:   1.0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateValue(tt.sliName, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
