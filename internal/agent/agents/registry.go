package agents

import (
	"github.com/Rubentxu/golem/internal/behavior"
)

// RegisterAgents registers all three vertical agents (UAT, Release, Security)
// with the behavior registry. Each agent is registered with kind=Agentic.
//
// Note: Agents are registered by their behavior definitions. Actual LLM, Frame,
// and Redactor injection happens at runtime through the behavior engine's
// AgenticContext. This function only registers the behavior metadata and
// handler signatures.
func RegisterAgents(reg *behavior.Registry) error {
	// Create lightweight behavior registration for each agent
	// Full initialization with real dependencies happens at runtime
	agents := []*behavior.Behavior{
		{
			ID:            UATAgentID,
			Version:       UATAgentVersion,
			Kind_:         behavior.KindAgentic,
			Subscriptions: []string{"requirement.defined.v1"},
			// AgenticH will be set by the runtime when the agent is actually used
		},
		{
			ID:            ReleaseAgentID,
			Version:       ReleaseAgentVersion,
			Kind_:         behavior.KindAgentic,
			Subscriptions: []string{"release.candidate.v1", "proposal.applied.v1"},
		},
		{
			ID:            SecurityAgentID,
			Version:       SecurityAgentVersion,
			Kind_:         behavior.KindAgentic,
			Subscriptions: []string{"vulnerability.detected.v1"},
		},
	}

	for _, a := range agents {
		if err := reg.Register(a); err != nil {
			return err
		}
	}
	return nil
}
