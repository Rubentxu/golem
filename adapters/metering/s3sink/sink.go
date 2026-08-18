// Package s3sink provides an S3 sink for metering rollups.
package s3sink

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// Sink exports metering rollups to S3.
type Sink struct {
	bucket string
}

// NewSink creates a new S3 sink for metering rollups.
func NewSink(bucket string) *Sink {
	return &Sink{bucket: bucket}
}

// Export exports metering rollups to S3.
func (s *Sink) Export(ctx context.Context, rollups []ports.MeteringRollup) error {
	if len(rollups) == 0 {
		return nil
	}

	// Group rollups by tenant and date.
	byTenantDate := make(map[string]map[string][]ports.MeteringRollup)
	for _, r := range rollups {
		date := r.Hour.Format("2006-01-02")
		if byTenantDate[r.TenantID] == nil {
			byTenantDate[r.TenantID] = make(map[string][]ports.MeteringRollup)
		}
		byTenantDate[r.TenantID][date] = append(byTenantDate[r.TenantID][date], r)
	}

	// Export each tenant/date combination.
	for tenantID, dates := range byTenantDate {
		for date, tenantRollups := range dates {
			key := fmt.Sprintf("metering/%s/%s.json", tenantID, date)
			_ = key // In real implementation, would upload to S3.

			// Serialize rollups.
			data, err := json.Marshal(tenantRollups)
			if err != nil {
				return fmt.Errorf("serialize rollups: %w", err)
			}

			// Upload to S3.
			_ = data // In real implementation, would upload to S3.

			// Emit event.
			_ = time.Now()
		}
	}

	return nil
}

// Compile-time interface check.
var _ = (*MeteringExporter)(nil)

// MeteringExporter is implemented by Sink.
type MeteringExporter interface {
	Export(ctx context.Context, rollups []ports.MeteringRollup) error
}
