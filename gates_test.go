package qsim

import (
	"math"
	"math/cmplx"
	"testing"
)

// TestGatesAreUnitary verifies g†g = I for every predefined gate.
func TestGatesAreUnitary(t *testing.T) {
	tests := []struct {
		name string
		g    Gate
	}{
		{"I", I}, {"X", X}, {"Y", Y}, {"Z", Z}, {"H", H}, {"S", S}, {"T", T},
		{"Rx(0.3)", Rx(0.3)},
		{"Ry(1.1)", Ry(1.1)},
		{"Rz(-2.5)", Rz(-2.5)},
		{"Phase(0.77)", Phase(0.77)},
		{"Rx(pi)", Rx(math.Pi)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := tt.g
			for i := 0; i < 2; i++ {
				for j := 0; j < 2; j++ {
					// (g†g)[i][j] = sum_k conj(g[k][i]) * g[k][j]
					var got complex128
					for k := 0; k < 2; k++ {
						got += cmplx.Conj(g[k][i]) * g[k][j]
					}
					want := complex128(0)
					if i == j {
						want = 1
					}
					if !cEq(got, want) {
						t.Errorf("(g†g)[%d][%d] = %v, want %v", i, j, got, want)
					}
				}
			}
		})
	}
}

// TestSelfInverseGates verifies the physics claim that H, X, Y, Z applied
// twice are the identity, and that rotation gates invert with negated angle.
// Gates are applied to a non-trivial superposition so phase errors show up.
func TestSelfInverseGates(t *testing.T) {
	tests := []struct {
		name   string
		g, inv Gate
	}{
		{"HH", H, H},
		{"XX", X, X},
		{"YY", Y, Y},
		{"ZZ", Z, Z},
		{"Rx then Rx(-θ)", Rx(0.9), Rx(-0.9)},
		{"Ry then Ry(-θ)", Ry(2.2), Ry(-2.2)},
		{"Rz then Rz(-θ)", Rz(-1.4), Rz(1.4)},
		{"Phase then Phase(-θ)", Phase(0.6), Phase(-0.6)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewState(2)
			if err != nil {
				t.Fatal(err)
			}
			// Prepare a state with complex, unequal amplitudes.
			s.ApplyGate(H, 0)
			s.ApplyGate(T, 0)
			s.ApplyGate(Ry(0.4), 1)
			before := s.Amplitudes()

			s.ApplyGate(tt.g, 0)
			s.ApplyGate(tt.inv, 0)
			wantAmps(t, s, before)
		})
	}
}

// TestGateCompositions verifies algebraic identities between the
// predefined gates: S² = Z, T² = S, T⁴ = Z, Phase(π) = Z on states,
// and HZH = X.
func TestGateCompositions(t *testing.T) {
	tests := []struct {
		name string
		lhs  []Gate // applied in order to qubit 0
		rhs  []Gate
	}{
		{"S²=Z", []Gate{S, S}, []Gate{Z}},
		{"T²=S", []Gate{T, T}, []Gate{S}},
		{"T⁴=Z", []Gate{T, T, T, T}, []Gate{Z}},
		{"Phase(π)=Z", []Gate{Phase(math.Pi)}, []Gate{Z}},
		{"HZH=X", []Gate{H, Z, H}, []Gate{X}},
		{"HXH=Z", []Gate{H, X, H}, []Gate{Z}},
		{"Phase(π/2)=S", []Gate{Phase(math.Pi / 2)}, []Gate{S}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mk := func(gates []Gate) *State {
				s, err := NewState(1)
				if err != nil {
					t.Fatal(err)
				}
				// Non-trivial start state.
				s.ApplyGate(Ry(0.8), 0)
				s.ApplyGate(Phase(0.3), 0)
				for _, g := range gates {
					s.ApplyGate(g, 0)
				}
				return s
			}
			left, right := mk(tt.lhs), mk(tt.rhs)
			wantAmps(t, left, right.Amplitudes())
		})
	}
}

// TestHOnZero verifies H|0> = (|0>+|1>)/√2 exactly (up to eps).
func TestHOnZero(t *testing.T) {
	s, err := NewState(1)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyGate(H, 0)
	wantAmps(t, s, []complex128{invSqrt2, invSqrt2})
}

// TestApplyGateTargetsCorrectQubit checks the little-endian index
// convention: X on qubit k maps |0...0> to the basis state with index 2^k.
func TestApplyGateTargetsCorrectQubit(t *testing.T) {
	for k := 0; k < 4; k++ {
		s, err := NewState(4)
		if err != nil {
			t.Fatal(err)
		}
		s.ApplyGate(X, k)
		want := make([]complex128, 16)
		want[1<<uint(k)] = 1
		wantAmps(t, s, want)
	}
}

// TestNormPreserved verifies unitarity numerically: a long random-ish gate
// sequence keeps the state normalized.
func TestNormPreserved(t *testing.T) {
	s, err := NewState(5)
	if err != nil {
		t.Fatal(err)
	}
	gates := []Gate{H, T, Rx(0.31), Ry(1.7), Rz(-0.9), S, Y, Phase(2.1)}
	for i := 0; i < 200; i++ {
		s.ApplyGate(gates[i%len(gates)], i%5)
	}
	if got := norm(s); math.Abs(got-1) > 1e-9 {
		t.Errorf("norm after 200 gates = %v, want 1", got)
	}
}
