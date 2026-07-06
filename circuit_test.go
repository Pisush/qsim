package qsim

import (
	"math"
	"math/rand"
	"sync"
	"testing"
)

// TestCircuitBellState verifies the fluent builder reproduces the Bell
// state: c.H(0).CNOT(0,1).
func TestCircuitBellState(t *testing.T) {
	c := NewCircuit(2)
	c.H(0).CNOT(0, 1)
	s, err := NewState(2)
	if err != nil {
		t.Fatal(err)
	}
	c.Run(s)
	wantAmps(t, s, []complex128{invSqrt2, 0, 0, invSqrt2})
}

// TestCircuitMatchesDirectCalls verifies every builder method appends the
// same operation its direct State counterpart applies.
func TestCircuitMatchesDirectCalls(t *testing.T) {
	tests := []struct {
		name   string
		build  func(c *Circuit)
		direct func(s *State)
	}{
		{"X", func(c *Circuit) { c.X(1) }, func(s *State) { s.ApplyGate(X, 1) }},
		{"Y", func(c *Circuit) { c.Y(0) }, func(s *State) { s.ApplyGate(Y, 0) }},
		{"Z", func(c *Circuit) { c.Z(2) }, func(s *State) { s.ApplyGate(Z, 2) }},
		{"H", func(c *Circuit) { c.H(1) }, func(s *State) { s.ApplyGate(H, 1) }},
		{"S", func(c *Circuit) { c.S(0) }, func(s *State) { s.ApplyGate(S, 0) }},
		{"T", func(c *Circuit) { c.T(2) }, func(s *State) { s.ApplyGate(T, 2) }},
		{"Rx", func(c *Circuit) { c.Rx(1, 0.4) }, func(s *State) { s.ApplyGate(Rx(0.4), 1) }},
		{"Ry", func(c *Circuit) { c.Ry(0, -1.1) }, func(s *State) { s.ApplyGate(Ry(-1.1), 0) }},
		{"Rz", func(c *Circuit) { c.Rz(2, 2.7) }, func(s *State) { s.ApplyGate(Rz(2.7), 2) }},
		{"Phase", func(c *Circuit) { c.Phase(1, 0.9) }, func(s *State) { s.ApplyGate(Phase(0.9), 1) }},
		{"CNOT", func(c *Circuit) { c.CNOT(0, 2) }, func(s *State) { s.ApplyControlled(X, 0, 2) }},
		{"CZ", func(c *Circuit) { c.CZ(2, 1) }, func(s *State) { s.ApplyControlled(Z, 2, 1) }},
		{"Apply", func(c *Circuit) { c.Apply(Ry(0.3), 0) }, func(s *State) { s.ApplyGate(Ry(0.3), 0) }},
		{"Controlled", func(c *Circuit) { c.Controlled(T, 1, 0) }, func(s *State) { s.ApplyControlled(T, 1, 0) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prep := func(s *State) { // shared non-trivial start state
				s.ApplyGate(H, 0)
				s.ApplyGate(Ry(0.6), 1)
				s.ApplyGate(H, 2)
				s.ApplyGate(T, 2)
			}
			viaCircuit, err := NewState(3)
			if err != nil {
				t.Fatal(err)
			}
			direct, err := NewState(3)
			if err != nil {
				t.Fatal(err)
			}
			prep(viaCircuit)
			prep(direct)

			c := NewCircuit(3)
			tt.build(c)
			c.Run(viaCircuit)
			tt.direct(direct)
			wantAmps(t, viaCircuit, direct.Amplitudes())
		})
	}
}

// TestCircuitFluentChaining verifies builder methods return the receiver
// and append in order.
func TestCircuitFluentChaining(t *testing.T) {
	c := NewCircuit(3)
	got := c.H(0).X(1).CNOT(0, 1).Rz(2, math.Pi/5).CZ(1, 2)
	if got != c {
		t.Error("fluent methods must return the receiver")
	}
	if c.Len() != 5 {
		t.Errorf("Len() = %d, want 5", c.Len())
	}
	if c.NumQubits() != 3 {
		t.Errorf("NumQubits() = %d, want 3", c.NumQubits())
	}
}

// TestCircuitReusable verifies a circuit is a pure description: running
// it twice on fresh states yields identical results.
func TestCircuitReusable(t *testing.T) {
	c := NewCircuit(2)
	c.H(0).CNOT(0, 1).T(1).Ry(0, 0.8)
	run := func() []complex128 {
		s, err := NewState(2)
		if err != nil {
			t.Fatal(err)
		}
		c.Run(s)
		return s.Amplitudes()
	}
	first, second := run(), run()
	for i := range first {
		if !cEq(first[i], second[i]) {
			t.Fatalf("amplitude[%d] differs between runs: %v vs %v", i, first[i], second[i])
		}
	}
}

// TestCircuitConcurrentRuns runs one built circuit on many independent
// states concurrently (meaningful under -race).
func TestCircuitConcurrentRuns(t *testing.T) {
	c := NewCircuit(5)
	for q := 0; q < 5; q++ {
		c.H(q)
	}
	c.CNOT(0, 1).CNOT(1, 2).CZ(2, 3).T(4)
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			s, err := NewState(5)
			if err != nil {
				t.Error(err)
				return
			}
			c.Run(s)
			s.MeasureAll(rand.New(rand.NewSource(seed)))
		}(int64(w))
	}
	wg.Wait()
}

func TestCircuitPanics(t *testing.T) {
	tests := []struct {
		name string
		call func()
	}{
		{"NewCircuit(0)", func() { NewCircuit(0) }},
		{"qubit out of range", func() { NewCircuit(2).H(2) }},
		{"negative qubit", func() { NewCircuit(2).X(-1) }},
		{"CNOT same qubit", func() { NewCircuit(2).CNOT(1, 1) }},
		{"CNOT control out of range", func() { NewCircuit(2).CNOT(2, 0) }},
		{"Run size mismatch", func() {
			s, err := NewState(3)
			if err != nil {
				panic(err)
			}
			NewCircuit(2).H(0).Run(s)
		}},
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
