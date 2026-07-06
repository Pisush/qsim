package qsim

import "fmt"

// ApplyControlled applies the single-qubit gate g to the target qubit,
// conditioned on the control qubit being |1>. ApplyControlled(X, c, t) is
// the CNOT gate; ApplyControlled(Z, c, t) is the CZ gate. The state is
// updated in place. It panics if either qubit index is out of range or if
// control == target.
func (s *State) ApplyControlled(g Gate, control, target int) {
	s.mustQubit(control)
	s.mustQubit(target)
	if control == target {
		panic(fmt.Sprintf("qsim: control and target must differ, both are %d", control))
	}
	applyControlledSerial(s.amps, g, control, target)
}

// applyControlledSerial applies g to the target bit of every amplitude
// pair whose control bit is set. The pair (i, i+stride) differs only in
// the target bit; since control != target, both elements share the same
// control bit, so a single mask test per pair suffices.
func applyControlledSerial(amps []complex128, g Gate, control, target int) {
	stride := 1 << uint(target)
	cmask := 1 << uint(control)
	for base := 0; base < len(amps); base += stride << 1 {
		for i := base; i < base+stride; i++ {
			if i&cmask == 0 {
				continue
			}
			a0, a1 := amps[i], amps[i+stride]
			amps[i] = g[0][0]*a0 + g[0][1]*a1
			amps[i+stride] = g[1][0]*a0 + g[1][1]*a1
		}
	}
}
