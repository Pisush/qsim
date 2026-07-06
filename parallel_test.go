package qsim

import (
	"math"
	"math/rand"
	"testing"
)

// randomAmps returns a normalized random state vector for n qubits.
func randomAmps(n int, rng *rand.Rand) []complex128 {
	amps := make([]complex128, 1<<uint(n))
	var norm float64
	for i := range amps {
		re, im := rng.NormFloat64(), rng.NormFloat64()
		amps[i] = complex(re, im)
		norm += re*re + im*im
	}
	scale := complex(1/math.Sqrt(norm), 0)
	for i := range amps {
		amps[i] *= scale
	}
	return amps
}

func TestPairIndex(t *testing.T) {
	// For every p, pairIndex(p, q) must have bit q clear, and the mapping
	// p -> pairIndex must enumerate exactly the indices with bit q clear,
	// in increasing order.
	const n = 6
	for q := 0; q < n; q++ {
		prev := -1
		for p := 0; p < 1<<(n-1); p++ {
			i0 := pairIndex(p, q)
			if i0>>uint(q)&1 != 0 {
				t.Fatalf("pairIndex(%d, %d) = %d has bit %d set", p, q, i0, q)
			}
			if i0 <= prev {
				t.Fatalf("pairIndex(%d, %d) = %d not increasing (prev %d)", p, q, i0, prev)
			}
			prev = i0
		}
	}
}

// TestParallelGateMatchesSerial verifies that the parallel path computes
// exactly what the serial path does, across qubit positions (including
// the top qubit, where the pair blocks are not contiguous chunks) and
// worker counts (including ones that do not divide the pair count).
func TestParallelGateMatchesSerial(t *testing.T) {
	const n = 15
	rng := rand.New(rand.NewSource(2024))
	base := randomAmps(n, rng)
	gates := []Gate{H, Y, T, Rx(0.37), Phase(1.9)}
	for _, qubit := range []int{0, 1, 7, 13, 14} {
		for _, workers := range []int{1, 2, 3, 7, 16} {
			g := gates[(qubit+workers)%len(gates)]
			serial := make([]complex128, len(base))
			parallel := make([]complex128, len(base))
			copy(serial, base)
			copy(parallel, base)

			applyGateSerial(serial, g, qubit)
			applyGateParallel(parallel, g, qubit, workers)

			for i := range serial {
				if !cEq(serial[i], parallel[i]) {
					t.Fatalf("qubit %d, workers %d: amplitude[%d] parallel %v != serial %v",
						qubit, workers, i, parallel[i], serial[i])
				}
			}
		}
	}
}

// TestParallelControlledMatchesSerial does the same for controlled gates,
// covering control<target, control>target, and extreme positions.
func TestParallelControlledMatchesSerial(t *testing.T) {
	const n = 15
	rng := rand.New(rand.NewSource(77))
	base := randomAmps(n, rng)
	pairs := []struct{ control, target int }{
		{0, 1}, {1, 0}, {0, 14}, {14, 0}, {6, 7}, {13, 14}, {14, 13},
	}
	for _, p := range pairs {
		for _, workers := range []int{1, 3, 8} {
			serial := make([]complex128, len(base))
			parallel := make([]complex128, len(base))
			copy(serial, base)
			copy(parallel, base)

			applyControlledSerial(serial, X, p.control, p.target)
			applyControlledParallel(parallel, X, p.control, p.target, workers)

			for i := range serial {
				if !cEq(serial[i], parallel[i]) {
					t.Fatalf("c=%d t=%d workers=%d: amplitude[%d] parallel %v != serial %v",
						p.control, p.target, workers, i, parallel[i], serial[i])
				}
			}
		}
	}
}

// TestParallelPathEndToEnd runs a real circuit on a state large enough to
// take the parallel path (2^14 amplitudes) and checks a physics
// invariant end to end. Meaningful under -race.
func TestParallelPathEndToEnd(t *testing.T) {
	const n = 14
	s, err := NewState(n)
	if err != nil {
		t.Fatal(err)
	}
	// H on every qubit, entangle a chain, then undo everything.
	for q := 0; q < n; q++ {
		s.ApplyGate(H, q)
	}
	for q := 0; q < n-1; q++ {
		s.ApplyControlled(X, q, q+1)
	}
	for q := n - 2; q >= 0; q-- {
		s.ApplyControlled(X, q, q+1)
	}
	for q := 0; q < n; q++ {
		s.ApplyGate(H, q)
	}
	amps := s.Amplitudes()
	if !cEq(amps[0], 1) {
		t.Errorf("amplitude[0] = %v, want 1 after inverse circuit", amps[0])
	}
	if !fEq(norm(s), 1) {
		t.Errorf("norm = %v, want 1", norm(s))
	}
}
