package otel

import "testing"

func TestProfileBuckets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		profile map[string]any
		wantLen int
		wantErr bool
	}{
		{
			name:    "nil profile",
			profile: nil,
			wantLen: len(DefaultHistogramBuckets),
		},
		{
			name:    "empty profile",
			profile: map[string]any{},
			wantLen: len(DefaultHistogramBuckets),
		},
		{
			name: "custom buckets",
			profile: map[string]any{
				"histogram_buckets": []any{1.0, 5.0, 10.0, 50.0, 100.0},
			},
			wantLen: 5,
		},
		{
			name: "mixed int and float",
			profile: map[string]any{
				"histogram_buckets": []any{1, 5.0, 10, 50.0},
			},
			wantLen: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProfileBuckets(tt.profile)
			if len(got) != tt.wantLen {
				t.Errorf("ProfileBuckets() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestValidateBuckets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		buckets []float64
		wantErr bool
	}{
		{"empty", []float64{}, false},
		{"valid", []float64{1, 5, 10}, false},
		{"single", []float64{100}, false},
		{"negative", []float64{-1, 5, 10}, true},
		{"zero", []float64{0, 5, 10}, true},
		{"not increasing", []float64{10, 5, 1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBuckets(tt.buckets)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBuckets() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBucketBounds(t *testing.T) {
	t.Parallel()

	buckets := []float64{10, 50, 100, 500}

	tests := []struct {
		value    float64
		expected int
	}{
		{5, -1},   // below first
		{10, 1},   // in second bucket [10, 50)
		{15, 1},   // in second bucket [10, 50)
		{50, 2},   // in third bucket [50, 100)
		{75, 2},   // in third bucket [50, 100)
		{100, 3},  // in fourth bucket [100, 500)
		{250, 3},  // in fourth bucket [100, 500)
		{500, 4},  // above last
		{1000, 4}, // above last
	}

	for _, tt := range tests {
		got := BucketBounds(buckets, tt.value)
		if got != tt.expected {
			t.Errorf("BucketBounds(%v) = %d, want %d", tt.value, got, tt.expected)
		}
	}
}

func TestQuantileBuckets(t *testing.T) {
	t.Parallel()

	buckets := QuantileBuckets()
	if len(buckets) == 0 {
		t.Error("QuantileBuckets() returned empty")
	}

	// Verify sorted and positive
	if err := ValidateBuckets(buckets); err != nil {
		t.Errorf("QuantileBuckets() produces invalid buckets: %v", err)
	}
}
