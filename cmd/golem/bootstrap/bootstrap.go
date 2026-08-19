// Package bootstrap provides factory functions that wire runtime.Options
// from profile.Profile. It lives in cmd/ so it can safely import adapters
// (ADR-045: adapter selection happens at the composition root).
package bootstrap

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Rubentxu/golem/adapters/cell/staticrouter"
	"github.com/Rubentxu/golem/adapters/checkpoint/memstore"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	"github.com/Rubentxu/golem/adapters/journal/bbolt"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	llmmem "github.com/Rubentxu/golem/adapters/llm/memstore"
	"github.com/Rubentxu/golem/adapters/metering"
	"github.com/Rubentxu/golem/adapters/observability/slo"
	pagingmemstore "github.com/Rubentxu/golem/adapters/paging/memstore"
	"github.com/Rubentxu/golem/adapters/paging/webhook"
	policymemstore "github.com/Rubentxu/golem/adapters/policy/memstore"
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	"github.com/Rubentxu/golem/adapters/registry/filesystem"
	searchmem "github.com/Rubentxu/golem/adapters/search/memstore"
	tenantcatalogmemstore "github.com/Rubentxu/golem/adapters/tenantcatalog/memstore"
	transportmem "github.com/Rubentxu/golem/adapters/transport/memstore"
	"github.com/Rubentxu/golem/adapters/transport/natsjs"
	"github.com/Rubentxu/golem/internal/application/runtime"
	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/internal/profile"
)

// NewRuntimeFromProfile constructs a Runtime from p.
// NATS transport degrades to memstore with WARN log when unreachable
// (2s timeout) — never fail-fast. ADR-057 §4.
func NewRuntimeFromProfile(p profile.Profile, obsbundle ports.Observability) (*runtime.Runtime, error) {
	opts, err := NewOptionsFromProfile(p, obsbundle)
	if err != nil {
		return nil, err
	}
	return runtime.New(opts)
}

// NewOptionsFromProfile builds runtime.Options from a profile.Profile.
// ADR-057 §4.
func NewOptionsFromProfile(p profile.Profile, obsbundle ports.Observability) (runtime.Options, error) {
	opts := runtime.Options{Obs: obsbundle}

	// Journal
	switch p.Adapter("journal") {
	case "bbolt":
		opts.Journal = newBboltJournal(p)
	case "memstore", "":
		opts.Journal = journalmem.NewJournal()
	default:
		return runtime.Options{}, errUnknownAdapter("journal", p.Adapter("journal"))
	}

	// Graph
	switch p.Adapter("graph") {
	case "memstore", "":
		opts.Graph = graphmem.NewGraph()
	default:
		return runtime.Options{}, errUnknownAdapter("graph", p.Adapter("graph"))
	}

	// Registry
	switch p.Adapter("registry") {
	case "memstore", "":
		opts.Registry = registrymem.NewRegistry()
	default:
		return runtime.Options{}, errUnknownAdapter("registry", p.Adapter("registry"))
	}

	// Transport (with NATS fallback)
	switch p.Adapter("transport") {
	case "natsjs":
		transport, ok := newNATSTransport(p)
		if !ok {
			log.Printf("profile=%s transport=memstore (warning: NATS unreachable at %s — falling back)",
				p.Name, os.Getenv("GOLEM_TEST_NATS_URL"))
			opts.Transport = transportmem.NewTransport()
		} else {
			opts.Transport = transport
		}
	case "memstore", "":
		opts.Transport = transportmem.NewTransport()
	default:
		return runtime.Options{}, errUnknownAdapter("transport", p.Adapter("transport"))
	}

	// Checkpoint
	switch p.Adapter("checkpoint") {
	case "memstore", "":
		opts.Checkpoint = memstore.NewCheckpoints()
	default:
		return runtime.Options{}, errUnknownAdapter("checkpoint", p.Adapter("checkpoint"))
	}

	// Search
	switch p.Adapter("search") {
	case "memstore", "":
		opts.Search = searchmem.NewSearch()
	default:
		return runtime.Options{}, errUnknownAdapter("search", p.Adapter("search"))
	}

	// LLM adapter (M7)
	switch p.Adapter("llm") {
	case "memstore", "":
		opts.LLM = llmmem.New(nil)
	default:
		return runtime.Options{}, errUnknownAdapter("llm", p.Adapter("llm"))
	}

	// Policy adapter (M7) — placeholder nil until PolicyEvaluator port is defined
	// Will be wired in T11 after PolicyEvaluator port is created
	opts.Policy = nil

	// CellRouter (M8)
	switch p.Adapter("cell-router") {
	case "staticrouter", "memstore", "":
		// Route only — cells are managed externally
		opts.CellRouter = staticrouter.NewRouter(nil)
	default:
		return runtime.Options{}, errUnknownAdapter("cell-router", p.Adapter("cell-router"))
	}

	// TenantCatalog (M8)
	switch p.Adapter("tenant-catalog") {
	case "memstore", "":
		opts.TenantCatalog = tenantcatalogmemstore.New()
	default:
		return runtime.Options{}, errUnknownAdapter("tenant-catalog", p.Adapter("tenant-catalog"))
	}

	// Quota (M8)
	switch p.Adapter("quota") {
	case "memstore", "":
		opts.QuotaEnforcer = policymemstore.NewQuotaStore()
	default:
		return runtime.Options{}, errUnknownAdapter("quota", p.Adapter("quota"))
	}

	// Meter (M8)
	switch p.Adapter("meter") {
	case "meter", "":
		opts.UsageMeter = metering.NewMeter()
	default:
		return runtime.Options{}, errUnknownAdapter("meter", p.Adapter("meter"))
	}

	// Paging (M8)
	switch p.Adapter("paging") {
	case "webhook":
		opts.Paging = webhook.NewClient("", "")
	case "memstore", "":
		opts.Paging = pagingmemstore.NewStore()
	default:
		return runtime.Options{}, errUnknownAdapter("paging", p.Adapter("paging"))
	}

	// SLO (M8)
	switch p.Adapter("slo") {
	case "slo", "":
		opts.SLOTracker = slo.NewTracker()
	default:
		return runtime.Options{}, errUnknownAdapter("slo", p.Adapter("slo"))
	}

	// AuthN (M8)
	switch p.Adapter("authn") {
	case "oidc":
		// OIDC config extracted from profile options if present
		opts.AuthN = &noOpAuthN{}
	case "memstore", "":
		// No-op AuthN for dev
		opts.AuthN = &noOpAuthN{}
	default:
		return runtime.Options{}, errUnknownAdapter("authn", p.Adapter("authn"))
	}

	// PackRegistry (M8)
	switch p.Adapter("pack_registry") {
	case "filesystem", "":
		opts.PackRegistry = filesystem.New(filesystem.DefaultRoot, nil, nil, nil)
	default:
		return runtime.Options{}, errUnknownAdapter("pack_registry", p.Adapter("pack_registry"))
	}

	return opts, nil
}

func errUnknownAdapter(port, kind string) error {
	return &unknownAdapterError{port: port, kind: kind}
}

type unknownAdapterError struct{ port, kind string }

func (e *unknownAdapterError) Error() string {
	return "profile: unknown " + e.port + " adapter " + e.kind
}

// newBboltJournal opens a bbolt journal from profile options.
func newBboltJournal(p profile.Profile) ports.JournalStore {
	path := "./var/golem.journal"
	if profileOpts := p.Option("bbolt"); profileOpts != nil {
		if v, ok := profileOpts["path"].(string); ok && v != "" {
			path = v
		}
	}
	s, err := bbolt.NewJournal(path, bbolt.Options{})
	if err != nil {
		log.Printf("profile=%s journal=bbolt: failed to open %s: %v — using memstore", p.Name, path, err)
		return journalmem.NewJournal()
	}
	return s
}

// newNATSTransport attempts NATS JetStream connection.
// 2s timeout per spec S24/S25; returns (nil, false) on failure.
func newNATSTransport(p profile.Profile) (ports.EventTransport, bool) {
	opts := p.Option("natsjs")
	url := os.Getenv("GOLEM_TEST_NATS_URL")
	if opts != nil {
		if v, ok := opts["url"].(string); ok && v != "" {
			url = v
		}
	}
	if url == "" {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	t, err := natsjs.Connect(ctx, natsjs.Config{URL: url})
	if err != nil {
		log.Printf("profile=%s transport=memstore (warning: NATS unreachable at %s — falling back)", p.Name, url)
		return nil, false
	}
	return t, true
}

// noOpAuthN implements ports.AuthN for dev/durable profiles where
// OIDC is not configured. It always returns an anonymous principal.
type noOpAuthN struct{}

func (n *noOpAuthN) VerifyBearer(ctx context.Context, token string) (ports.Principal, error) {
	_ = token
	return ports.Principal{Subject: "anonymous", Type: "user"}, nil
}

func (n *noOpAuthN) Discover(ctx context.Context) (string, error) {
	_ = ctx
	return "", nil
}

// Ensure noOpAuthN implements ports.AuthN
var _ ports.AuthN = (*noOpAuthN)(nil)
