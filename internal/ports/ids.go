package ports

import "time"

// Clock is the time port. Domain and application code never call time.Now
// directly; the clock is injected so tests are deterministic (TEST_STRATEGY:
// "Clock/random inyectados").
type Clock interface {
	Now() time.Time
}

// IDGenerator is the identity port. Reference format is ULID (sortable,
// 128-bit, encoded as 26 Crockford base32 characters) but the contract only
// requires unique, lexically sortable identifiers.
type IDGenerator interface {
	NewID() string
}
