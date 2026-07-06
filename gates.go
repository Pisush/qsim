package qsim

import (
	"math"
	"math/cmplx"
)

// Gate is a single-qubit gate: a 2x2 unitary matrix in row-major order,
// acting on the amplitude pair (a0, a1) of a qubit as
//
//	a0' = g[0][0]*a0 + g[0][1]*a1
//	a1' = g[1][0]*a0 + g[1][1]*a1
type Gate [2][2]complex128

var invSqrt2 = complex(1/math.Sqrt2, 0)

// Predefined single-qubit gates.
var (
	// I is the identity gate.
	I = Gate{{1, 0}, {0, 1}}
	// X is the Pauli-X (NOT) gate: it swaps |0> and |1>.
	X = Gate{{0, 1}, {1, 0}}
	// Y is the Pauli-Y gate.
	Y = Gate{{0, -1i}, {1i, 0}}
	// Z is the Pauli-Z gate: it flips the phase of |1>.
	Z = Gate{{1, 0}, {0, -1}}
	// H is the Hadamard gate: it maps |0> and |1> to equal superpositions.
	H = Gate{{invSqrt2, invSqrt2}, {invSqrt2, -invSqrt2}}
	// S is the phase gate, a 90-degree rotation about the Z axis (sqrt of Z).
	S = Gate{{1, 0}, {0, 1i}}
	// T is the pi/8 gate, a 45-degree rotation about the Z axis (sqrt of S).
	T = Gate{{1, 0}, {0, cmplx.Exp(complex(0, math.Pi/4))}}
)

// Rx returns the rotation gate about the X axis by angle theta (radians).
func Rx(theta float64) Gate {
	c := complex(math.Cos(theta/2), 0)
	s := complex(0, -math.Sin(theta/2))
	return Gate{{c, s}, {s, c}}
}

// Ry returns the rotation gate about the Y axis by angle theta (radians).
func Ry(theta float64) Gate {
	c := complex(math.Cos(theta/2), 0)
	s := complex(math.Sin(theta/2), 0)
	return Gate{{c, -s}, {s, c}}
}

// Rz returns the rotation gate about the Z axis by angle theta (radians).
func Rz(theta float64) Gate {
	return Gate{
		{cmplx.Exp(complex(0, -theta/2)), 0},
		{0, cmplx.Exp(complex(0, theta/2))},
	}
}

// Phase returns the phase-shift gate diag(1, e^(i*theta)). It equals Rz(theta)
// up to a global phase; Phase(pi/2) = S and Phase(pi/4) = T.
func Phase(theta float64) Gate {
	return Gate{{1, 0}, {0, cmplx.Exp(complex(0, theta))}}
}
