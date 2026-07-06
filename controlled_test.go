package qsim

import (
	"math/rand"
	"testing"
)

// TestCNOTTruthTable verifies CNOT on every 2-qubit basis state, for both
// control/target orderings.
func TestCNOTTruthTable(t *testing.T) {
	tests := []struct {
		name            string
		control, target int
		in, want        int // basis-state indices (bit k = qubit k)
	}{
		{"c0t1 |00>", 0, 1, 0b00, 0b00},
		{"c0t1 |01>", 0, 1, 0b01, 0b11}, // control (qubit 0) set: flip qubit 1
		{"c0t1 |10>", 0, 1, 0b10, 0b10},
		{"c0t1 |11>", 0, 1, 0b11, 0b01},
		{"c1t0 |00>", 1, 0, 0b00, 0b00},
		{"c1t0 |01>", 1, 0, 0b01, 0b01},
		{"c1t0 |10>", 1, 0, 0b10, 0b11}, // control (qubit 1) set: flip qubit 0
		{"c1t0 |11>", 1, 0, 0b11, 0b10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewState(2)
			if err != nil {
				t.Fatal(err)
			}
			// Prepare the input basis state.
			for q := 0; q < 2; q++ {
				if tt.in>>uint(q)&1 == 1 {
					s.ApplyGate(X, q)
				}
			}
			s.ApplyControlled(X, tt.control, tt.target)
			want := make([]complex128, 4)
			want[tt.want] = 1
			wantAmps(t, s, want)
		})
	}
}

// TestCNOTSelfInverse verifies CNOT·CNOT = I on a non-trivial state.
func TestCNOTSelfInverse(t *testing.T) {
	s, err := NewState(3)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyGate(H, 0)
	s.ApplyGate(Ry(0.7), 1)
	s.ApplyGate(T, 2)
	before := s.Amplitudes()
	s.ApplyControlled(X, 0, 2)
	s.ApplyControlled(X, 0, 2)
	wantAmps(t, s, before)
}

// TestControlUnsetIsIdentity verifies that a controlled gate does nothing
// when the control qubit is |0>.
func TestControlUnsetIsIdentity(t *testing.T) {
	s, err := NewState(2)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyGate(H, 1) // control qubit 0 stays |0>
	before := s.Amplitudes()
	s.ApplyControlled(X, 0, 1)
	wantAmps(t, s, before)
}

// TestCZSymmetric verifies the physics fact that CZ is symmetric in
// control and target.
func TestCZSymmetric(t *testing.T) {
	mk := func(control, target int) *State {
		s, err := NewState(2)
		if err != nil {
			t.Fatal(err)
		}
		s.ApplyGate(H, 0)
		s.ApplyGate(Ry(1.2), 1)
		s.ApplyControlled(Z, control, target)
		return s
	}
	a, b := mk(0, 1), mk(1, 0)
	wantAmps(t, a, b.Amplitudes())
}

// TestBellStateAmplitudes verifies H(0) then CNOT(0,1) yields
// (|00> + |11>)/√2.
func TestBellStateAmplitudes(t *testing.T) {
	s, err := NewState(2)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyGate(H, 0)
	s.ApplyControlled(X, 0, 1)
	wantAmps(t, s, []complex128{invSqrt2, 0, 0, invSqrt2})
}

// TestBellStateCorrelation is the milestone-2 physics claim: measuring
// both qubits of a Bell state gives perfectly correlated outcomes in
// every trial, with each branch appearing ~50% of the time.
func TestBellStateCorrelation(t *testing.T) {
	const trials = 10000
	rng := rand.New(rand.NewSource(1234))
	ones := 0
	for i := 0; i < trials; i++ {
		s, err := NewState(2)
		if err != nil {
			t.Fatal(err)
		}
		s.ApplyGate(H, 0)
		s.ApplyControlled(X, 0, 1)
		out := s.MeasureAll(rng)
		if out[0] != out[1] {
			t.Fatalf("trial %d: Bell state measured anti-correlated outcome %v", i, out)
		}
		ones += out[0]
	}
	// Binomial(10000, 0.5): sd = 50. Allow 5 sigma.
	if ones < 4750 || ones > 5250 {
		t.Errorf("|11> branch in %d/%d trials, want ~5000 (outside 5-sigma band)", ones, trials)
	}
}

// TestBellStateCollapse verifies that measuring one half of a Bell pair
// makes the other half definite.
func TestBellStateCollapse(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	for i := 0; i < 200; i++ {
		s, err := NewState(2)
		if err != nil {
			t.Fatal(err)
		}
		s.ApplyGate(H, 0)
		s.ApplyControlled(X, 0, 1)
		got := s.Measure(0, rng)
		if p := s.Probability(1); !fEq(p, float64(got)) {
			t.Fatalf("after measuring qubit 0 = %d, Probability(1) = %v, want %v", got, p, float64(got))
		}
	}
}

// TestGHZState verifies a 3-qubit GHZ state built from H + two CNOTs:
// (|000> + |111>)/√2, with perfect three-way measurement correlation.
func TestGHZState(t *testing.T) {
	s, err := NewState(3)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyGate(H, 0)
	s.ApplyControlled(X, 0, 1)
	s.ApplyControlled(X, 1, 2)
	want := make([]complex128, 8)
	want[0b000] = invSqrt2
	want[0b111] = invSqrt2
	wantAmps(t, s, want)

	rng := rand.New(rand.NewSource(5))
	for i := 0; i < 500; i++ {
		g, err := NewState(3)
		if err != nil {
			t.Fatal(err)
		}
		g.ApplyGate(H, 0)
		g.ApplyControlled(X, 0, 1)
		g.ApplyControlled(X, 1, 2)
		out := g.MeasureAll(rng)
		if out[0] != out[1] || out[1] != out[2] {
			t.Fatalf("trial %d: GHZ state measured uncorrelated outcome %v", i, out)
		}
	}
}

// TestControlledPhaseKickback verifies phase kickback: with the target in
// |->, CNOT flips the control's phase (H on control turns that into a bit
// flip), i.e. the standard Deutsch-style interference circuit.
func TestControlledPhaseKickback(t *testing.T) {
	s, err := NewState(2)
	if err != nil {
		t.Fatal(err)
	}
	// Target (qubit 1) in |->, control (qubit 0) in |+>.
	s.ApplyGate(X, 1)
	s.ApplyGate(H, 1)
	s.ApplyGate(H, 0)
	s.ApplyControlled(X, 0, 1)
	s.ApplyGate(H, 0)
	// Kickback: control must now read 1 with certainty.
	if p := s.Probability(0); !fEq(p, 1) {
		t.Errorf("Probability(control=1) after kickback = %v, want 1", p)
	}
}

func TestApplyControlledPanics(t *testing.T) {
	s, err := NewState(3)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		call func()
	}{
		{"control out of range", func() { s.ApplyControlled(X, 3, 0) }},
		{"target out of range", func() { s.ApplyControlled(X, 0, -1) }},
		{"control equals target", func() { s.ApplyControlled(X, 1, 1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("expected panic, got none")
				}
			}()
			tt.call()
		})
	}
}
