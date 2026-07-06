DRAFT

# qsim: a quantum simulator that's mostly just slices and goroutines

qsim is a state-vector quantum circuit simulator written in Go. It represents
the state of an *n*-qubit register as a `[]complex128` of length 2^n — one
complex amplitude per possible bit string — and applies gates by rewriting
that slice directly. No tensor products, no 2^n×2^n matrices, no external
math dependencies. Just a slice, some complex arithmetic, and (once the slice
gets big enough) a worker pool.

The one clever idea is how gates are applied. Textbook quantum simulation
builds a full 2^n×2^n unitary matrix for every gate — mostly zeros, wired up
via tensor product with identity matrices for every qubit the gate doesn't
touch — and multiplies. qsim skips that entirely. A single-qubit gate only
ever needs to mix *pairs* of amplitudes whose indices differ in exactly one
bit (the target qubit's bit), by a 2×2 matrix. So `applyGateSerial` walks the
slice with a `stride`, grabs each pair `(amps[i], amps[i+stride])`, and
replaces both with the gate's linear combination. That's O(2^n) work instead
of O(4^n), and — this is the part that makes it fun to build in Go — every
pair is independent of every other pair. There's no shared state to
serialize, so `parallel.go` hands disjoint ranges of pairs to
`runtime.NumCPU()` goroutines and lets them run with no locks, no atomics,
just a `sync.WaitGroup` at the end. Below 2^14 amplitudes (14 qubits) it
falls back to the serial loop, because goroutine setup costs more than the
arithmetic it would parallelize — a threshold you can watch cross over in the
package's own benchmarks.

Why it's cool: this is quantum computing made *legible*. `ApplyControlled`
is the same trick with one extra mask check for CNOT and CZ. `Circuit` is a
fluent builder — `qsim.NewCircuit(2).H(0).CNOT(0, 1)` — that's a reusable
description you can `Run` on any fresh `State`, which is exactly what
`cmd/grover` needs to sample thousands of runs cheaply. And Grover's search
— the algorithm that finds a marked item among N candidates in O(√N) instead
of O(N) — falls out of three primitives already in the package: `FlipPhase`
as the oracle, `H` on every qubit for diffusion, and `Measure` at the end.
Run `go run ./cmd/grover -n 10 -marked 42` and watch the success probability
climb toward its theoretical peak at `floor(π/4·√N)` iterations, then
*decline* if you keep going — a detail that surprises people the first time
they see the curve turn over.

It caps out at 26 qubits by default (a full gibibyte of amplitudes), because
that's the honest price of simulating something exponential exactly. That
ceiling is the whole reason real quantum hardware is worth building — and
also why watching this simulator hit it, on a laptop, in Go, is worth ten
minutes of anyone's time.
