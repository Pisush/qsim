package qsim

import "fmt"

// Circuit is an ordered sequence of gate operations on a fixed number of
// qubits, built fluently and applied to a State with Run:
//
//	c := qsim.NewCircuit(2)
//	c.H(0).CNOT(0, 1)
//	s, _ := qsim.NewState(2)
//	c.Run(s) // s is now the Bell state (|00> + |11>)/√2
//
// A Circuit is a reusable description: Run may be called on any number of
// (fresh) states. Building a Circuit is not safe for concurrent use, but
// a fully built Circuit may Run on independent states concurrently.
type Circuit struct {
	nQubits int
	ops     []circuitOp
}

// circuitOp is one gate application; control is -1 for single-qubit ops.
type circuitOp struct {
	gate            Gate
	control, target int
}

// NewCircuit returns an empty circuit over nQubits qubits. It panics if
// nQubits is not positive; unlike a State, a Circuit allocates no
// state vector, so there is no memory cap to enforce.
func NewCircuit(nQubits int) *Circuit {
	if nQubits < 1 {
		panic(fmt.Sprintf("qsim: circuit must have at least 1 qubit, got %d", nQubits))
	}
	return &Circuit{nQubits: nQubits}
}

// NumQubits returns the number of qubits the circuit operates on.
func (c *Circuit) NumQubits() int { return c.nQubits }

// Len returns the number of gate operations in the circuit.
func (c *Circuit) Len() int { return len(c.ops) }

func (c *Circuit) mustQubit(q int) {
	if q < 0 || q >= c.nQubits {
		panic(fmt.Sprintf("qsim: qubit index %d out of range [0,%d)", q, c.nQubits))
	}
}

// Apply appends an arbitrary single-qubit gate on qubit q.
func (c *Circuit) Apply(g Gate, q int) *Circuit {
	c.mustQubit(q)
	c.ops = append(c.ops, circuitOp{gate: g, control: -1, target: q})
	return c
}

// Controlled appends g on the target qubit, conditioned on control.
// It panics if control == target or either index is out of range.
func (c *Circuit) Controlled(g Gate, control, target int) *Circuit {
	c.mustQubit(control)
	c.mustQubit(target)
	if control == target {
		panic(fmt.Sprintf("qsim: control and target must differ, both are %d", control))
	}
	c.ops = append(c.ops, circuitOp{gate: g, control: control, target: target})
	return c
}

// X appends a Pauli-X (NOT) gate on qubit q.
func (c *Circuit) X(q int) *Circuit { return c.Apply(X, q) }

// Y appends a Pauli-Y gate on qubit q.
func (c *Circuit) Y(q int) *Circuit { return c.Apply(Y, q) }

// Z appends a Pauli-Z gate on qubit q.
func (c *Circuit) Z(q int) *Circuit { return c.Apply(Z, q) }

// H appends a Hadamard gate on qubit q.
func (c *Circuit) H(q int) *Circuit { return c.Apply(H, q) }

// S appends a phase gate (sqrt of Z) on qubit q.
func (c *Circuit) S(q int) *Circuit { return c.Apply(S, q) }

// T appends a T gate (sqrt of S) on qubit q.
func (c *Circuit) T(q int) *Circuit { return c.Apply(T, q) }

// Rx appends a rotation about the X axis by theta radians on qubit q.
func (c *Circuit) Rx(q int, theta float64) *Circuit { return c.Apply(Rx(theta), q) }

// Ry appends a rotation about the Y axis by theta radians on qubit q.
func (c *Circuit) Ry(q int, theta float64) *Circuit { return c.Apply(Ry(theta), q) }

// Rz appends a rotation about the Z axis by theta radians on qubit q.
func (c *Circuit) Rz(q int, theta float64) *Circuit { return c.Apply(Rz(theta), q) }

// Phase appends a phase shift diag(1, e^(i*theta)) on qubit q.
func (c *Circuit) Phase(q int, theta float64) *Circuit { return c.Apply(Phase(theta), q) }

// CNOT appends a controlled-NOT gate.
func (c *Circuit) CNOT(control, target int) *Circuit { return c.Controlled(X, control, target) }

// CZ appends a controlled-Z gate.
func (c *Circuit) CZ(control, target int) *Circuit { return c.Controlled(Z, control, target) }

// Run applies the circuit's operations to s in order. It panics if the
// state's qubit count does not match the circuit's.
func (c *Circuit) Run(s *State) {
	if s.NumQubits() != c.nQubits {
		panic(fmt.Sprintf("qsim: circuit over %d qubits cannot run on a %d-qubit state", c.nQubits, s.NumQubits()))
	}
	for _, op := range c.ops {
		if op.control < 0 {
			s.ApplyGate(op.gate, op.target)
		} else {
			s.ApplyControlled(op.gate, op.control, op.target)
		}
	}
}
