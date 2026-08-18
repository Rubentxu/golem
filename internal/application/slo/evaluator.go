// Package slo provides the SLO evaluation service (REQ-SLO-004).
package slo

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// Evaluator runs the SLO evaluation loop every 60 seconds.
// It evaluates all registered SLOs and pages operators when alerts fire.
type Evaluator struct {
	tracker  ports.SLOTracker
	pager    ports.Paging
	journal  ports.JournalStore
	logger   *slog.Logger
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// EvaluatorConfig configures the SLO evaluator.
type EvaluatorConfig struct {
	Tracker  ports.SLOTracker
	Pager    ports.Paging
	Journal  ports.JournalStore
	Logger   *slog.Logger
	Interval time.Duration
}

// NewEvaluator creates a new SLO evaluator.
func NewEvaluator(cfg EvaluatorConfig) *Evaluator {
	if cfg.Interval == 0 {
		cfg.Interval = 60 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Evaluator{
		tracker:  cfg.Tracker,
		pager:    cfg.Pager,
		journal:  cfg.Journal,
		logger:   cfg.Logger,
		interval: cfg.Interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the evaluation loop. It runs until Stop is called.
func (e *Evaluator) Start(ctx context.Context) {
	e.wg.Add(1)
	go e.runLoop(ctx)
}

// Stop gracefully stops the evaluation loop.
func (e *Evaluator) Stop() {
	close(e.stopCh)
	e.wg.Wait()
}

func (e *Evaluator) runLoop(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	// Run once at startup
	e.evaluate(ctx)

	for {
		select {
		case <-ticker.C:
			e.evaluate(ctx)
		case <-e.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Evaluate runs one evaluation cycle and pages on-call for any violations.
func (e *Evaluator) Evaluate(ctx context.Context) {
	e.evaluate(ctx)
}

func (e *Evaluator) evaluate(ctx context.Context) {
	violations, err := e.tracker.Evaluate(ctx)
	if err != nil {
		e.logger.Warn("SLO evaluate error", "err", err)
		return
	}

	for _, v := range violations {
		e.logger.Warn("SLO violation",
			"slo", v.SLOName,
			"budget_consumed", v.BudgetConsumed,
			"burn_rate", v.BurnRate,
		)

		// Page on-call for critical violations
		if v.BudgetConsumed >= 1.0 {
			e.pageAlert(ctx, v, ports.AlertSeverityCritical,
				"SLO budget exhausted: "+v.SLOName)
		} else if v.BurnRate > 2.0 {
			e.pageAlert(ctx, v, ports.AlertSeverityHigh,
				"SLO budget burn: "+v.SLOName+" at "+formatBurnRate(v.BurnRate))
		}
	}
}

func (e *Evaluator) pageAlert(ctx context.Context, v ports.SLOViolation, severity ports.AlertSeverity, msg string) {
	if e.pager == nil {
		return
	}

	alert := ports.Alert{
		Severity: severity,
		Route:    "slo-alert",
		Message:  msg,
		SLIName:  v.SLOName,
	}

	if err := e.pager.Page(ctx, alert); err != nil {
		e.logger.Warn("Failed to page SLO alert", "err", err, "slo", v.SLOName)
	}
}

func formatBurnRate(r float64) string {
	return "undefined" // placeholder
}
