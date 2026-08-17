// Package application hosts the use-case layer of GOLEM: commands and
// queries orchestrating domain logic behind the hexagonal ports (CQRS-style
// separation, ADR-006).
//
// Write path: Request → AuthN/AuthZ → Command → Domain validation →
// Journal append → Accepted Event → Outbox/Projection → Async reactions.
// Read path: Query → Tenant Scope → Query Budget → Projection → Stable DTO.
//
// Application code depends on ports, never on adapters or vendor types.
package application
