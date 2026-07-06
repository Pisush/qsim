// Package qsim is a state-vector quantum circuit simulator.
//
// The state of n qubits is stored as a normalized []complex128 of length
// 2^n. The package uses a little-endian qubit convention: qubit k
// corresponds to bit k of the amplitude index, so amplitude index i
// describes the basis state in which qubit k reads (i>>k)&1.
//
// Gates are applied by mixing pairs of amplitudes in place; no 2^n x 2^n
// matrices are ever built. A State is not safe for concurrent use, but
// independent State values may be simulated concurrently: the package
// keeps no global mutable state.
package qsim

import (
	"fmt"
	"math"
	"math/rand"
)

// DefaultQubitCap is the maximum number of qubits NewState accepts unless
// overridden with WithQubitCap. 26 qubits require 2^26 amplitudes, i.e.
// 1 GiB of complex128 values.
const DefaultQubitCap = 26

// State is the full state vector of an n-qubit register.
//
// A State is not safe for concurrent use by multiple goroutines, but
// distinct State values are fully independent and may be used from
// different goroutines simultaneously.
type State struct {
	amps    []complex128
	nQubits int
}

// Option configures NewState.
type Option func(*stateConfig)

type stateConfig struct {
	qubitCap int
}

// WithQubitCap overrides the maximum number of qubits NewState accepts.
// Use with care: memory doubles per qubit (2^n amplitudes of 16 bytes).
func WithQubitCap(maxQubits int) Option {
	return func(c *stateConfig) { c.qubitCap = maxQubits }
}

// NewState returns a new nQubits-qubit register initialized to |00...0>.
// It returns an error if nQubits is not positive or exceeds the qubit cap
// (DefaultQubitCap unless overridden with WithQubitCap).
func NewState(nQubits int, opts ...Option) (*State, error) {
	cfg := stateConfig{qubitCap: DefaultQubitCap}
	for _, opt := range opts {
		opt(&cfg)
	}
	if nQubits < 1 {
		return nil, fmt.Errorf("qsim: number of qubits must be at least 1, got %d", nQubits)
	}
	if nQubits > cfg.qubitCap {
		return nil, fmt.Errorf("qsim: %d qubits exceeds the cap of %d (a state vector needs 16*2^%d bytes; raise the cap with WithQubitCap if you have the memory)",
			nQubits, cfg.qubitCap, nQubits)
	}
	amps := make([]complex128, 1<<uint(nQubits))
	amps[0] = 1
	return &State{amps: amps, nQubits: nQubits}, nil
}

// NumQubits returns the number of qubits in the register.
func (s *State) NumQubits() int { return s.nQubits }

// Amplitudes returns a copy of the state vector. Index i holds the
// amplitude of the basis state in which qubit k reads (i>>k)&1.
func (s *State) Amplitudes() []complex128 {
	out := make([]complex128, len(s.amps))
	copy(out, s.amps)
	return out
}

// mustQubit panics if qubit is not a valid index for this register.
// An out-of-range qubit index is a programmer error, not a runtime
// condition, so it panics rather than returning an error.
func (s *State) mustQubit(qubit int) {
	if qubit < 0 || qubit >= s.nQubits {
		panic(fmt.Sprintf("qsim: qubit index %d out of range [0,%d)", qubit, s.nQubits))
	}
}

// ApplyGate applies the single-qubit gate g to the given qubit, updating
// the state in place. It panics if qubit is out of range.
func (s *State) ApplyGate(g Gate, qubit int) {
	s.mustQubit(qubit)
	applyGateSerial(s.amps, g, qubit)
}

// applyGateSerial applies g to the given qubit of the state vector amps.
// Amplitude pairs whose indices differ only in bit `qubit` are mixed by
// the 2x2 matrix g.
func applyGateSerial(amps []complex128, g Gate, qubit int) {
	stride := 1 << uint(qubit)
	for base := 0; base < len(amps); base += stride << 1 {
		for i := base; i < base+stride; i++ {
			a0, a1 := amps[i], amps[i+stride]
			amps[i] = g[0][0]*a0 + g[0][1]*a1
			amps[i+stride] = g[1][0]*a0 + g[1][1]*a1
		}
	}
}

// Probability returns the probability that measuring the given qubit
// yields 1. It panics if qubit is out of range.
func (s *State) Probability(qubit int) float64 {
	s.mustQubit(qubit)
	var p1 float64
	stride := 1 << uint(qubit)
	for base := stride; base < len(s.amps); base += stride << 1 {
		for i := base; i < base+stride; i++ {
			a := s.amps[i]
			p1 += real(a)*real(a) + imag(a)*imag(a)
		}
	}
	// Guard against accumulated floating-point drift.
	return math.Min(math.Max(p1, 0), 1)
}

// Measure measures the given qubit in the computational basis, collapses
// the state accordingly, and returns the outcome (0 or 1). Randomness
// comes exclusively from rng, so runs are reproducible with a seeded
// source. It panics if qubit is out of range or rng is nil.
func (s *State) Measure(qubit int, rng *rand.Rand) int {
	s.mustQubit(qubit)
	if rng == nil {
		panic("qsim: Measure requires a non-nil *rand.Rand")
	}
	p1 := s.Probability(qubit)
	outcome := 0
	pKeep := 1 - p1
	if rng.Float64() < p1 {
		outcome = 1
		pKeep = p1
	}
	// Collapse: zero the amplitudes inconsistent with the outcome and
	// renormalize the rest.
	scale := complex(1/math.Sqrt(pKeep), 0)
	for i := range s.amps {
		if (i>>uint(qubit))&1 == outcome {
			s.amps[i] *= scale
		} else {
			s.amps[i] = 0
		}
	}
	return outcome
}

// MeasureAll measures every qubit in order 0..n-1, collapsing the state
// to a single basis state, and returns the outcomes indexed by qubit.
// It panics if rng is nil.
func (s *State) MeasureAll(rng *rand.Rand) []int {
	if rng == nil {
		panic("qsim: MeasureAll requires a non-nil *rand.Rand")
	}
	out := make([]int, s.nQubits)
	for q := 0; q < s.nQubits; q++ {
		out[q] = s.Measure(q, rng)
	}
	return out
}
