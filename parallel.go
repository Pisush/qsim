package qsim

import (
	"runtime"
	"sync"
)

// parallelThreshold is the state-vector length (in amplitudes) below
// which gates are applied serially: for small states, goroutine startup
// and synchronization cost more than the arithmetic they would spread.
const parallelThreshold = 1 << 14

// pairIndex returns the index of the lower member of the p-th amplitude
// pair for a gate on the given qubit: it inserts a 0 bit at position
// qubit into p. The upper member is pairIndex | stride.
//
// Enumerating pairs by p (0 <= p < len(amps)/2) instead of by blocks
// keeps parallel work perfectly balanced for every qubit position,
// including the top qubit where there is only one contiguous block.
func pairIndex(p, qubit int) int {
	mask := 1<<uint(qubit) - 1
	return (p&^mask)<<1 | p&mask
}

// applyGateParallel applies g to the given qubit, splitting the
// len(amps)/2 amplitude pairs across a pool of workers goroutines. Each
// pair is owned by exactly one worker, so no synchronization on the
// amplitudes is needed beyond the final Wait.
func applyGateParallel(amps []complex128, g Gate, qubit, workers int) {
	half := len(amps) / 2
	stride := 1 << uint(qubit)
	chunk := (half + workers - 1) / workers
	var wg sync.WaitGroup
	for lo := 0; lo < half; lo += chunk {
		hi := lo + chunk
		if hi > half {
			hi = half
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			for p := lo; p < hi; p++ {
				i0 := pairIndex(p, qubit)
				i1 := i0 | stride
				a0, a1 := amps[i0], amps[i1]
				amps[i0] = g[0][0]*a0 + g[0][1]*a1
				amps[i1] = g[1][0]*a0 + g[1][1]*a1
			}
		}(lo, hi)
	}
	wg.Wait()
}

// applyControlledParallel is applyGateParallel restricted to pairs whose
// control bit is set.
func applyControlledParallel(amps []complex128, g Gate, control, target, workers int) {
	half := len(amps) / 2
	stride := 1 << uint(target)
	cmask := 1 << uint(control)
	chunk := (half + workers - 1) / workers
	var wg sync.WaitGroup
	for lo := 0; lo < half; lo += chunk {
		hi := lo + chunk
		if hi > half {
			hi = half
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			for p := lo; p < hi; p++ {
				i0 := pairIndex(p, target)
				if i0&cmask == 0 {
					continue
				}
				i1 := i0 | stride
				a0, a1 := amps[i0], amps[i1]
				amps[i0] = g[0][0]*a0 + g[0][1]*a1
				amps[i1] = g[1][0]*a0 + g[1][1]*a1
			}
		}(lo, hi)
	}
	wg.Wait()
}

// numWorkers returns the worker-pool size for parallel gate application.
func numWorkers() int { return runtime.NumCPU() }
