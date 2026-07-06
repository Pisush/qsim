package qsim

import (
	"fmt"
	"testing"
)

// benchGate measures repeated application of H at nQubits qubits, cycling
// the target qubit so every stride pattern is exercised. H is unitary and
// self-inverse, so amplitudes stay bounded no matter how many iterations
// the benchmark runs.
func benchGate(b *testing.B, nQubits int, apply func(amps []complex128, g Gate, qubit int)) {
	amps := make([]complex128, 1<<uint(nQubits))
	amps[0] = 1
	b.SetBytes(int64(16 << uint(nQubits)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		apply(amps, H, i%nQubits)
	}
}

// BenchmarkApplyGate compares the serial and parallel gate kernels at 20
// and 22 qubits (the spec's "20+ qubits" comparison point) and documents
// where the parallel path starts paying off.
func BenchmarkApplyGate(b *testing.B) {
	workers := numWorkers()
	for _, n := range []int{14, 20, 22} {
		b.Run(fmt.Sprintf("serial/%dq", n), func(b *testing.B) {
			benchGate(b, n, applyGateSerial)
		})
		b.Run(fmt.Sprintf("parallel/%dq", n), func(b *testing.B) {
			benchGate(b, n, func(amps []complex128, g Gate, qubit int) {
				applyGateParallel(amps, g, qubit, workers)
			})
		})
	}
}

// BenchmarkApplyControlled compares the serial and parallel controlled
// kernels at 20 qubits.
func BenchmarkApplyControlled(b *testing.B) {
	workers := numWorkers()
	const n = 20
	b.Run("serial/20q", func(b *testing.B) {
		amps := make([]complex128, 1<<n)
		amps[0] = 1
		b.SetBytes(16 << n)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			applyControlledSerial(amps, X, i%n, (i+1)%n)
		}
	})
	b.Run("parallel/20q", func(b *testing.B) {
		amps := make([]complex128, 1<<n)
		amps[0] = 1
		b.SetBytes(16 << n)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			applyControlledParallel(amps, X, i%n, (i+1)%n, workers)
		}
	})
}
