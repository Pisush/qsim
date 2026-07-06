package qsim

import (
	"math"
	"math/cmplx"
	"math/rand"
	"strings"
	"sync"
	"testing"
)

const eps = 1e-9

func cEq(a, b complex128) bool { return cmplx.Abs(a-b) < eps }
func fEq(a, b float64) bool    { return math.Abs(a-b) < eps }

// wantAmps fails the test unless the state's amplitudes match want within eps.
func wantAmps(t *testing.T, s *State, want []complex128) {
	t.Helper()
	got := s.Amplitudes()
	if len(got) != len(want) {
		t.Fatalf("amplitude count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !cEq(got[i], want[i]) {
			t.Errorf("amplitude[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// norm returns the squared norm of the state vector.
func norm(s *State) float64 {
	var n float64
	for _, a := range s.Amplitudes() {
		n += real(a)*real(a) + imag(a)*imag(a)
	}
	return n
}

func TestNewState(t *testing.T) {
	tests := []struct {
		name    string
		nQubits int
		opts    []Option
		wantErr bool
		wantLen int
	}{
		{name: "one qubit", nQubits: 1, wantLen: 2},
		{name: "three qubits", nQubits: 3, wantLen: 8},
		{name: "at default cap", nQubits: DefaultQubitCap, wantLen: 1 << DefaultQubitCap},
		{name: "zero qubits", nQubits: 0, wantErr: true},
		{name: "negative qubits", nQubits: -2, wantErr: true},
		{name: "above default cap", nQubits: DefaultQubitCap + 1, wantErr: true},
		{name: "above custom cap", nQubits: 5, opts: []Option{WithQubitCap(4)}, wantErr: true},
		{name: "within custom cap", nQubits: 4, opts: []Option{WithQubitCap(4)}, wantLen: 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.nQubits > 20 && testing.Short() {
				t.Skip("large allocation in -short mode")
			}
			s, err := NewState(tt.nQubits, tt.opts...)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewState(%d) error = nil, want error", tt.nQubits)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewState(%d) unexpected error: %v", tt.nQubits, err)
			}
			if s.NumQubits() != tt.nQubits {
				t.Errorf("NumQubits() = %d, want %d", s.NumQubits(), tt.nQubits)
			}
			amps := s.Amplitudes()
			if len(amps) != tt.wantLen {
				t.Fatalf("len(Amplitudes()) = %d, want %d", len(amps), tt.wantLen)
			}
			if !cEq(amps[0], 1) {
				t.Errorf("amplitude[0] = %v, want 1", amps[0])
			}
			if !fEq(norm(s), 1) {
				t.Errorf("norm = %v, want 1", norm(s))
			}
		})
	}
}

func TestCapErrorMessage(t *testing.T) {
	_, err := NewState(30)
	if err == nil {
		t.Fatal("NewState(30) error = nil, want cap error")
	}
	if !strings.Contains(err.Error(), "cap") {
		t.Errorf("cap error %q does not mention the cap", err)
	}
}

func TestAmplitudesReturnsCopy(t *testing.T) {
	s, err := NewState(1)
	if err != nil {
		t.Fatal(err)
	}
	amps := s.Amplitudes()
	amps[0] = 0 // must not affect the state
	if !cEq(s.Amplitudes()[0], 1) {
		t.Error("mutating the Amplitudes() result changed the state")
	}
}

func TestProbability(t *testing.T) {
	tests := []struct {
		name  string
		build func(s *State)
		qubit int
		want  float64
	}{
		{name: "fresh state P(1)=0", build: func(s *State) {}, qubit: 0, want: 0},
		{name: "after X P(1)=1", build: func(s *State) { s.ApplyGate(X, 1) }, qubit: 1, want: 1},
		{name: "after H P(1)=0.5", build: func(s *State) { s.ApplyGate(H, 0) }, qubit: 0, want: 0.5},
		{name: "other qubit unaffected", build: func(s *State) { s.ApplyGate(H, 0) }, qubit: 2, want: 0},
		{
			name:  "Ry(pi/3) gives sin^2(pi/6)",
			build: func(s *State) { s.ApplyGate(Ry(math.Pi/3), 0) },
			qubit: 0,
			want:  0.25,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewState(3)
			if err != nil {
				t.Fatal(err)
			}
			tt.build(s)
			if got := s.Probability(tt.qubit); !fEq(got, tt.want) {
				t.Errorf("Probability(%d) = %v, want %v", tt.qubit, got, tt.want)
			}
		})
	}
}

// TestHMeasurementFiftyFifty checks the physics claim that H|0> measures
// 0 and 1 each with probability 1/2, by sampling many independent trials
// with a seeded RNG.
func TestHMeasurementFiftyFifty(t *testing.T) {
	const trials = 10000
	rng := rand.New(rand.NewSource(42))
	ones := 0
	for i := 0; i < trials; i++ {
		s, err := NewState(1)
		if err != nil {
			t.Fatal(err)
		}
		s.ApplyGate(H, 0)
		ones += s.Measure(0, rng)
	}
	// Binomial(10000, 0.5): sd = 50. Allow 5 sigma.
	if ones < 4750 || ones > 5250 {
		t.Errorf("measured 1 in %d/%d trials, want ~5000 (outside 5-sigma band)", ones, trials)
	}
}

func TestMeasureCollapses(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 100; trial++ {
		s, err := NewState(2)
		if err != nil {
			t.Fatal(err)
		}
		s.ApplyGate(H, 0)
		s.ApplyGate(H, 1)
		got := s.Measure(0, rng)
		// After collapse the outcome is definite and the state normalized.
		if p := s.Probability(0); !fEq(p, float64(got)) {
			t.Fatalf("after measuring %d, Probability(0) = %v, want %v", got, p, float64(got))
		}
		if !fEq(norm(s), 1) {
			t.Fatalf("state not normalized after measurement: norm = %v", norm(s))
		}
		// Re-measuring must reproduce the outcome.
		for i := 0; i < 5; i++ {
			if again := s.Measure(0, rng); again != got {
				t.Fatalf("re-measurement gave %d after initial %d", again, got)
			}
		}
	}
}

func TestMeasureDeterministicOutcomes(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	s, err := NewState(2)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyGate(X, 1)
	for i := 0; i < 10; i++ {
		if got := s.Measure(0, rng); got != 0 {
			t.Fatalf("Measure(0) on |10> = %d, want 0", got)
		}
		if got := s.Measure(1, rng); got != 1 {
			t.Fatalf("Measure(1) on |10> = %d, want 1", got)
		}
	}
}

func TestMeasureAll(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	s, err := NewState(3)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyGate(X, 0)
	s.ApplyGate(X, 2)
	got := s.MeasureAll(rng)
	want := []int{1, 0, 1}
	for q := range want {
		if got[q] != want[q] {
			t.Errorf("MeasureAll()[%d] = %d, want %d", q, got[q], want[q])
		}
	}
	// The state must have collapsed to exactly |101> (index 0b101 = 5).
	wantState := make([]complex128, 8)
	wantState[5] = 1
	wantAmps(t, s, wantState)
}

func TestQubitIndexPanics(t *testing.T) {
	s, err := NewState(2)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(1))
	tests := []struct {
		name string
		call func()
	}{
		{"ApplyGate negative", func() { s.ApplyGate(X, -1) }},
		{"ApplyGate too large", func() { s.ApplyGate(X, 2) }},
		{"Probability too large", func() { s.Probability(5) }},
		{"Measure negative", func() { s.Measure(-3, rng) }},
		{"Measure nil rng", func() { s.Measure(0, nil) }},
		{"MeasureAll nil rng", func() { s.MeasureAll(nil) }},
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

// TestConcurrentIndependentSimulations exercises the guarantee that
// distinct State values can be simulated concurrently (run with -race).
func TestConcurrentIndependentSimulations(t *testing.T) {
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			s, err := NewState(6)
			if err != nil {
				t.Error(err)
				return
			}
			for i := 0; i < 100; i++ {
				s.ApplyGate(H, i%6)
				s.ApplyGate(T, (i+1)%6)
			}
			s.MeasureAll(rng)
		}(int64(w))
	}
	wg.Wait()
}
