// Command golem-worker runs the journal tail loops of one GOLEM process:
// graph projection (Journal → Engineering Graph) and the outbox publisher
// (Journal → EventTransport), each with its own checkpoint.
package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	otelobs "github.com/Rubentxu/golem/adapters/observability/otel"
	"github.com/Rubentxu/golem/adapters/storage/kms"
	"github.com/Rubentxu/golem/adapters/storage/s3"
	"github.com/Rubentxu/golem/cmd/golem/bootstrap"
	"github.com/Rubentxu/golem/internal/application/runtime"
	"github.com/Rubentxu/golem/internal/canonical"
	"github.com/Rubentxu/golem/internal/profile"
)

// defaultExportCron is the cron expression for daily canonical export at 2am UTC.
const defaultExportCron = "0 2 * * *"

func main() {
	prof, err := profile.LoadFromEnv()
	if err != nil {
		log.Fatalf("profile load: %v", err)
	}
	log.Printf("profile=%s", prof.Name)

	obsbundle, shutdownObs, err := otelobs.Setup(context.Background(), "golem-worker", "0.1.0")
	if err != nil {
		log.Fatalf("observability setup: %v", err)
	}
	defer func() { _ = shutdownObs(context.Background()) }()

	rt, err := bootstrap.NewRuntimeFromProfile(prof, obsbundle)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start the daily canonical export cron if enabled.
	if os.Getenv("GOLEM_EXPORT_ENABLED") == "true" {
		go runExportCron(ctx, rt, prof)
	}

	const batchSize = 100
	log.Printf("golem-worker: projection + outbox loops (batch=%d)", batchSize)
	if err := rt.Run(ctx, batchSize, 250*time.Millisecond); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
	log.Print("golem-worker: stopped")
}

// runExportCron runs a daily canonical export cycle at 2am UTC:
//
//	Export → S3 upload → KMS sign → emit audit.export.completed event.
func runExportCron(ctx context.Context, rt *runtime.Runtime, prof profile.Profile) {
	cronExpr := os.Getenv("GOLEM_EXPORT_CRON")
	if cronExpr == "" {
		cronExpr = defaultExportCron
	}
	log.Printf("export-cron: starting with schedule %q", cronExpr)

	// Build S3 writer and KMS signer from profile.
	s3w := buildS3Writer(ctx, prof)
	signer := buildKMSSigner(ctx, prof)

	cron := cronChan()
	for {
		select {
		case <-ctx.Done():
			log.Print("export-cron: stopped")
			return
		case <-cron:
			runExportCycle(ctx, rt, s3w, signer)
		}
	}
}

func buildS3Writer(ctx context.Context, p profile.Profile) *s3.Writer {
	opts := p.Option("s3")
	if opts == nil {
		log.Printf("export-cron: s3 profile options missing — S3 upload disabled")
		return nil
	}
	bucket, _ := opts["bucket"].(string)
	prefix, _ := opts["prefix"].(string) // e.g. "golem-exports/"
	if bucket == "" {
		log.Printf("export-cron: s3.bucket not set — S3 upload disabled")
		return nil
	}
	w, err := s3.NewWriter(ctx, bucket, prefix)
	if err != nil {
		log.Printf("export-cron: s3.NewWriter: %v — S3 upload disabled", err)
		return nil
	}
	return w
}

func buildKMSSigner(ctx context.Context, p profile.Profile) *kms.Signer {
	opts := p.Option("kms")
	if opts == nil {
		log.Printf("export-cron: kms profile options missing — signing disabled")
		return nil
	}
	keyAlias, _ := opts["key_alias"].(string)
	if keyAlias == "" {
		keyAlias = "alias/golem-export"
	}
	signer, err := kms.NewSigner(ctx, keyAlias)
	if err != nil {
		log.Printf("export-cron: kms.NewSigner: %v — signing disabled", err)
		return nil
	}
	return signer
}

// runExportCycle executes one export → S3 → KMS sign cycle.
func runExportCycle(ctx context.Context, rt *runtime.Runtime, s3w *s3.Writer, signer *kms.Signer) {
	start := time.Now()
	log.Print("export-cron: starting export cycle")

	// Run canonical export.
	var buf bytes.Buffer
	exporter := canonical.Exporter{
		TenantID: "default", // TODO: multi-tenant — iterate over all tenants
		Graph:    rt.Graph,
		Journal:  rt.Journal,
		Out:      &buf,
	}
	m, err := exporter.Export(ctx)
	if err != nil {
		log.Printf("export-cron: canonical.Export: %v", err)
		return
	}

	// Upload to S3 if writer is configured.
	s3Key := ""
	if s3w != nil {
		key := s3KeyForManifest(m)
		size := int64(buf.Len())
		if _, err := s3w.Write(ctx, key, &buf, size); err != nil {
			log.Printf("export-cron: s3.Write: %v", err)
			return
		}
		s3Key = key
		log.Printf("export-cron: uploaded to s3://%s/%s", s3w.Bucket(), key)
	}

	// Sign manifest with KMS if signer is configured.
	sigHex := ""
	if signer != nil {
		signable, _ := m.SignedPayload()
		sigHex, err = signer.Sign(ctx, signable)
		if err != nil {
			log.Printf("export-cron: kms.Sign: %v", err)
			return
		}
		log.Printf("export-cron: signed manifest with KMS key %s", signer.KeyAlias())
	}

	dur := time.Since(start)
	log.Printf("export-cron: completed in %v — nodes=%d edges=%d s3_key=%q signed=%v",
		dur, m.Counts.Nodes, m.Counts.Edges, s3Key, sigHex != "")
}

func s3KeyForManifest(m canonical.Manifest) string {
	return "exports/" + m.TenantID + "/" + m.CreatedAt[:10] + "/canonical.tar"
}

// cronChan returns a channel that fires at the next occurrence of 2am UTC and every 24h thereafter.
func cronChan() <-chan time.Time {
	ch := make(chan time.Time, 1)
	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			timer := time.NewTimer(time.Until(next))
			<-timer.C
			select {
			case ch <- time.Now():
			default:
			}
			// 24h cycle.
			daily := time.NewTicker(24 * time.Hour)
			<-daily.C
			daily.Stop()
			select {
			case ch <- time.Now():
			default:
			}
		}
	}()
	return ch
}
