// Package clock provides the reference implementation of the Clock port.
package clock

import (
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// SystemClock reads the real wall clock.
type SystemClock struct{}

// Now implements ports.Clock.
func (SystemClock) Now() time.Time { return time.Now() }

// Fixed returns a clock that always reports t; for tests.
func Fixed(t time.Time) ports.Clock { return fixed(t) }

type fixed time.Time

func (f fixed) Now() time.Time { return time.Time(f) }

var _ ports.Clock = SystemClock{}
