package tck

import (
	"github.com/Rubentxu/golem/internal/api/httpapi"
	"github.com/Rubentxu/golem/internal/application/runtime"
	"github.com/Rubentxu/golem/internal/obs"
	"github.com/Rubentxu/golem/internal/ports"
)

// NewMountSet builds the full set of HTTPMounts for the given runtime.
func NewMountSet(rt *runtime.Runtime) []httpapi.HTTPMount {
	return []httpapi.HTTPMount{
		&httpapi.WorkMount{},
		httpapi.NewReleaseMount(),
		httpapi.NewVerificationMount(),
		httpapi.NewPlatformMount(nil),
		httpapi.NewAdminMount(nil, ports.Observability{}),
	}
}

// MountDepsForRT builds MountDeps from a runtime.Runtime with all available ports wired.
func MountDepsForRT(rt *runtime.Runtime) httpapi.MountDeps {
	return httpapi.MountDeps{
		Observability:             obs.Fill(ports.Observability{}),
		Bus:                       rt.Bus,
		GraphStore:                rt.Graph,
		GraphNodeFetcher:          ports.NewGraphNodeFetcherOverGraphStore(rt.Graph),
		JournalStreamReader:       ports.NewJournalStreamReaderOverJournalStore(rt.Journal),
		Journal:                  rt.Journal,
		EntityRefReader:           ports.NewEntityRefReaderOverGraphStore(rt.Graph),
		WorkItemReader:            nil,
		WorkItemWriter:            nil,
		SCMStreamReader:           nil,
		ArtifactReader:            nil,
		ReleaseGraphReader:        nil,
		SupplyChainEvidenceReader: nil,
		BlastRadiusQuery:          nil,
		TestRunReader:             nil,
	}
}
