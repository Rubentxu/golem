// Package agents implements the three vertical agentic behaviors of M7:
// UAT agent, Release agent, and Security agent. Each is a behavior v2 with
// kind=Agentic, driven by an LLM and a lens over the graph.
//
// Each agent follows the same pattern:
//   - Subscribes to a specific event type
//   - Uses a lens to gather graph context (compile-time spec, not user-inputed)
//   - Renders a static prompt template (compile-time constant, no user interpolation)
//   - Calls LLM via the injected LLMProvider port
//   - Returns a ProposalNote with operations for privileged mutations
//
// No agent performs direct privileged writes — all mutations go through the
// Proposal lifecycle (propose → policy gate → human approval → apply).
package agents
