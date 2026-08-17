// Package ids provides the reference ULID generator behind the IDGenerator
// port. Implemented with the standard library only so it can live in
// internal/ (ADR-045: no third-party SDKs inside internal/).
//
// ULID: 128 bits — 48-bit millisecond timestamp + 80 bits of crypto/rand
// entropy, encoded as 26 Crockford base32 characters. Lexically sortable,
// which makes event ids order-correlated with time.
package ids

import (
	"crypto/rand"
	"encoding/binary"
	"strings"
	"sync"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

const (
	// Alphabet is Crockford base32 (I, L, O, U excluded).
	Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	// ULIDLen is the fixed encoded length: 10 (timestamp) + 16 (entropy).
	ULIDLen = 26
)

var (
	mu      sync.Mutex
	lastMS  uint64
	lastEnt [10]byte
)

// New returns a ULID for the given instant. IDs generated within the same
// millisecond are strictly monotonic (entropy increment), so a single
// process never emits duplicate or decreasing ULIDs.
func New(now time.Time) string {
	ms := uint64(now.UnixMilli())

	mu.Lock()
	var e [10]byte
	if _, err := rand.Read(e[:]); err != nil {
		// crypto/rand failure means the system entropy source is broken;
		// generating IDs anyway would silently violate uniqueness.
		mu.Unlock()
		panic("ids: crypto/rand unavailable: " + err.Error())
	}
	if ms == lastMS && !greater(e, lastEnt) {
		addOne(&lastEnt)
		e = lastEnt
	} else {
		lastEnt = e
	}
	lastMS = ms
	mu.Unlock()

	return encode(ms, e)
}

// ULIDGenerator adapts the generator to the IDGenerator port using the
// injected clock (never time.Now directly).
type ULIDGenerator struct {
	Clock ports.Clock
}

// NewGenerator builds a generator over the given clock.
func NewGenerator(c ports.Clock) *ULIDGenerator {
	return &ULIDGenerator{Clock: c}
}

// NewID implements ports.IDGenerator.
func (g *ULIDGenerator) NewID() string {
	return New(g.Clock.Now())
}

func encode(ms uint64, e [10]byte) string {
	var b strings.Builder
	b.Grow(ULIDLen)
	for i := 0; i < 10; i++ {
		b.WriteByte(Alphabet[(ms>>(45-5*i))&0x1f])
	}
	hi := binary.BigEndian.Uint64(e[0:8])
	lo := uint64(binary.BigEndian.Uint16(e[8:10]))
	for i := 0; i < 16; i++ {
		b.WriteByte(Alphabet[bits5(hi, lo, i)])
	}
	return b.String()
}

// bits5 extracts the i-th group of 5 bits (MSB-first) from the 80-bit
// value hi:lo.
func bits5(hi, lo uint64, i int) byte {
	var v uint64
	for b := 0; b < 5; b++ {
		bit := i*5 + b
		v <<= 1
		if bit < 64 {
			v |= (hi >> (63 - bit)) & 1
		} else {
			v |= (lo >> (79 - bit)) & 1
		}
	}
	return byte(v)
}

func greater(a, b [10]byte) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func addOne(e *[10]byte) {
	for i := len(e) - 1; i >= 0; i-- {
		e[i]++
		if e[i] != 0 {
			return
		}
	}
}
