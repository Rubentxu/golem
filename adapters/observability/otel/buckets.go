// Package otel provides OTel SDK setup and bridges to ports.Observability.
package otel

import "math"

// DefaultHistogramBuckets are the default OTel histogram buckets for latency.
var DefaultHistogramBuckets = []float64{
	0.5, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 30000, 60000,
}

// P50P95P99Buckets are optimized for P50/P95/P99 latency measurements.
var P50P95P99Buckets = []float64{
	5, 10, 25, 50, 100, 150, 200, 250, 300, 400, 500, 750, 1000,
}

// ProfileBuckets returns histogram buckets from a profile, falling back to defaults.
func ProfileBuckets(profile map[string]any) []float64 {
	if profile == nil {
		return DefaultHistogramBuckets
	}

	bucketsRaw, ok := profile["histogram_buckets"]
	if !ok {
		return DefaultHistogramBuckets
	}

	bucketsList, ok := bucketsRaw.([]any)
	if !ok || len(bucketsList) == 0 {
		return DefaultHistogramBuckets
	}

	buckets := make([]float64, 0, len(bucketsList))
	for _, b := range bucketsList {
		switch v := b.(type) {
		case float64:
			if v > 0 {
				buckets = append(buckets, v)
			}
		case int:
			if v > 0 {
				buckets = append(buckets, float64(v))
			}
		}
	}

	if len(buckets) == 0 {
		return DefaultHistogramBuckets
	}
	return buckets
}

// ValidateBuckets checks that buckets are positive and sorted.
func ValidateBuckets(buckets []float64) error {
	if len(buckets) == 0 {
		return nil
	}
	for i, b := range buckets {
		if b <= 0 {
			return &bucketError{idx: i, msg: "bucket must be positive"}
		}
		if i > 0 && b <= buckets[i-1] {
			return &bucketError{idx: i, msg: "buckets must be strictly increasing"}
		}
	}
	return nil
}

// bucketError describes a histogram bucket validation error.
type bucketError struct {
	idx int
	msg string
}

func (e *bucketError) Error() string {
	return e.msg
}

// BucketBounds computes the bucket index for a value.
// Returns -1 if value is below first bucket.
func BucketBounds(buckets []float64, value float64) int {
	if len(buckets) == 0 || value < buckets[0] {
		return -1
	}
	for i, b := range buckets {
		if value < b {
			return i
		}
	}
	return len(buckets)
}

// QuantileBuckets returns the bucket boundaries that span the given quantiles.
// For example, QuantileBuckets(0.5, 0.95, 0.99) returns buckets suitable for P50/P95/P99.
func QuantileBuckets(quantiles ...float64) []float64 {
	// Use P50/P95/P99 defaults if no quantiles specified
	if len(quantiles) == 0 {
		return P50P95P99Buckets
	}

	// Build bucket list from quantile targets
	maxLatency := 60000.0 // 60 seconds max
	buckets := make([]float64, 0, len(quantiles)*3)

	for _, q := range quantiles {
		if q <= 0 || q >= 1 {
			continue
		}
		// Map quantile to latency bucket
		// For P50, target ~50ms; for P95, target ~250ms; for P99, target ~500ms
		var target float64
		switch {
		case q <= 0.5:
			target = q * 100 // P50 → 50ms
		case q <= 0.95:
			target = (q-0.5)*1000 + 50 // P95 → 500ms
		case q <= 0.99:
			target = (q-0.95)*5000 + 500 // P99 → 700ms
		default:
			target = (q-0.99)*50000 + 1000 // P99.9 → ~5500ms
		}

		// Add buckets around the target
		target = math.Min(target, maxLatency)
		buckets = append(buckets,
			target*0.5,
			target*0.8,
			target,
			target*1.2,
		)
	}

	return buckets
}
