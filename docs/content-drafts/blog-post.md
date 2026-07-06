> DRAFT — technical blog post for qsim (github.com/Pisush/qsim)

# Building a parallel quantum simulator in Go

Quantum computers don't exist on your laptop, but their math does. If you
write down the state of an *n*-qubit system as a vector of 2^n complex
numbers and figure out how gates rewrite that vector, you can simulate any
quantum circuit exactly — you just pay for it in memory and CPU. qsim is a
state-vector simulator that does exactly that, in Go, and it turns out Go's
concurrency primitives map onto the problem unusually well.

This post walks through the two ideas that make qsim work — applying
gates without ever building a matrix, and splitting that work across
goroutines with no locks — then looks at Grover's search, the demo that
makes the whole thing feel real.

## The state is just a slice

An *n*-qubit register lives in `state.go` as a `[]complex128` of length
`1<<n`. Index `i` is a basis state, and bit `k` of `i` tells you what
qubit `k` reads in that basis state — a little-endian convention baked
into every gate function in the package. `NewState(n)` allocates the
slice and sets `amps[0] = 1`: the all-zeros state, everything else zero.

That's it. There's no wrapper type for "a qubit," no tensor-product
machinery. Entanglement between qubits is just correlations among the
amplitudes at different indices, which is also why the vector can't be
factored back into per-qubit pieces once qubits are entangled — the
representation is unavoidably exponential.

The size cap matters in practice. Doubling the qubit count doubles the
memory, so 24 qubits is 256 MiB of amplitudes and 26 is a full gibibyte.
`NewState` refuses anything past `DefaultQubitCap` (26) unless you
override it with `WithQubitCap` — a guard rail against an `-n 40` typo
that would otherwise try to allocate 16 exbibytes instead of failing
fast.

## Gates without matrices

The textbook way to apply a single-qubit gate is to build its 2^n × 2^n
matrix (mostly zeros, via tensor product with n−1 identity matrices) and
multiply. qsim doesn't, for the same reason nobody multiplies by an
identity matrix to leave a number unchanged: the big matrix carries no
information the 2×2 gate didn't already have. What the gate on qubit `q`
*actually* does is pair up amplitudes whose indices differ only in bit
`q`, and mix each pair by the gate's 2×2 matrix:

```go
func applyGateSerial(amps []complex128, g Gate, qubit int) {
    stride := 1 << uint(qubit)
    for base := 0; base < len(amps); base += stride << 1 {
        for i := base; i < base+stride; i++ {
            a0, a1 := amps[i], amps[i+stride]
            amps[i] = g[0][0]*a0 + g[0][1]*a1
            amps[i+stride] = g[1][0]*a0 + g[1][1]*a1
        }
    }
}
```

`stride` is the distance between the two halves of a pair — `1<<qubit` —
and the double loop walks every pair exactly once. This is O(2^n) work
per gate instead of O(2^n · 2^n) work-with-mostly-zeros, and it's also
the representation you'd want to hand to a worker pool, because every
pair is independent of every other pair. `ApplyControlled`
(`controlled.go`) is the same trick with one extra mask check: skip the
pair unless the control bit is set, since a controlled gate only touches
amplitudes where the control qubit reads 1.

`Gate` itself is refreshingly small — a `[2][2]complex128` — and the
predefined gates in `gates.go` (`X, Y, Z, H, S, T`, plus parameterized
`Rx`, `Ry`, `Rz`, `Phase`) are each a couple of lines of complex
arithmetic. `Phase(θ)` is `diag(1, e^{iθ})`; `Rz(θ)` is the same gate up
to a global phase, so `Phase(π/2)` behaves like `S` and `Phase(π/4)`
like `T` even though the matrices aren't bit-identical.

## Why goroutines fit state-vector partitioning

Go's answer to "spread this work across cores" is goroutines plus a
`sync.WaitGroup`, and state-vector simulation is about as friendly a fit
as numerical code gets: the amplitude pairs for a given gate are disjoint
by construction, so there is no shared mutable state to protect once
you've handed a range of pairs to a goroutine. No mutexes, no atomics.

qsim's `parallel.go` enumerates pairs by index `p` in `[0, len(amps)/2)`
rather than by contiguous block, using a small bit trick — `pairIndex`
inserts a zero bit at position `qubit` into `p` to get the lower member
of the p-th pair. That keeps the work perfectly balanced across workers
for *every* qubit position, including the top qubit, where a block-based
split would hand one goroutine a giant contiguous half and everyone else
nothing. `applyGateParallel` then chunks `[0, half)` evenly across
`runtime.NumCPU()` goroutines and waits.

There's a threshold below which this isn't worth it — `parallelThreshold`
is `1<<14` amplitudes (14 qubits). Below that, goroutine startup and
`sync.WaitGroup` overhead cost more than the arithmetic they'd spread, so
`ApplyGate` falls back to the serial loop. The README's own benchmarks
bear this out: at 20 qubits, `ApplyGate` drops from 1.82ms serial to
0.36ms parallel on an 8-core Apple Silicon machine, and at 22 qubits from
4.2ms to 1.7ms. `ApplyControlled` shows the same pattern at a smaller
margin, since roughly half its pairs are skipped by the control-bit check
regardless of worker count.

One design choice worth calling out: a `*State` is *not* safe for
concurrent use by multiple goroutines simultaneously, but the package
keeps zero global mutable state, so independent `State` values can be
simulated on different goroutines with no coordination at all. The test
suite runs clean under `go test -race`.

## Circuits, fluently

Building a circuit gate-by-gate against a raw `*State` gets verbose fast,
so `circuit.go` adds a small builder:

```go
c := qsim.NewCircuit(2)
c.H(0).CNOT(0, 1)

s, _ := qsim.NewState(2)
c.Run(s) // s is now the Bell state (|00> + |11>)/√2
```

A `Circuit` is an ordered list of `circuitOp` values (a gate plus a
target, and a control if it's a controlled operation); it allocates no
state vector itself. That makes a `Circuit` reusable — a description you
can `Run` against any number of fresh states, which is exactly what the
sampling in Grover's search needs.

## Watching Grover's probability climb

Grover's search is qsim's flagship demo because it's the algorithm that
makes "quantum speedup" visible on a chart instead of just asserted.
Classically, finding one marked element among *N* candidates takes O(N)
queries on average. Grover's algorithm does it in O(√N) by repeatedly
applying an oracle (a phase flip on the marked amplitude) followed by a
diffusion operator (inversion about the mean), amplifying the marked
amplitude's magnitude a little more each round.

The `grover` package (`grover/grover.go`) builds this directly on qsim's
primitives. The oracle is one call to `s.FlipPhase(marked)` — qsim's
`FlipPhase` negates a single basis amplitude, which is precisely the
"mark one element" primitive Grover's algorithm needs. Diffusion is
realized as `H^n`, a phase flip on `|0...0⟩`, then `H^n` again — three
lines of `ApplyGate` calls per iteration:

```go
func iterate(s *qsim.State, marked int) {
    s.FlipPhase(marked) // oracle
    n := s.NumQubits()
    for q := 0; q < n; q++ {
        s.ApplyGate(qsim.H, q)
    }
    s.FlipPhase(0)
    for q := 0; q < n; q++ {
        s.ApplyGate(qsim.H, q)
    }
}
```

The success probability after *k* iterations has a closed form,
sin²((2k+1)θ) with sin θ = 1/√N, and it peaks at `k = floor(π/4·√N)`
iterations — `grover.OptimalIterations` computes exactly that — and then
*decreases* if you keep going past the peak, which is a detail that
trips people up the first time they see it: more iterations is not always
better. `cmd/grover` charts this directly:

```sh
go run ./cmd/grover -n 10 -marked 42
```

prints the probability after every iteration as a small ASCII bar chart,
climbing toward the peak and then declining past it, then samples 1,000
actual measurements at the optimal iteration count to check the empirical
hit rate against the theoretical prediction. Watching the curve rise and
fall from a "quantum" object that's really just an in-place rewrite of a
`[]complex128` is, honestly, most of the fun of this project.

## What this is and isn't

qsim is a pure-state, state-vector simulator: no noise model, no density
matrices, no error correction, and gates are single- or
controlled-single-qubit only — a Toffoli has to be decomposed, since
there's no native multi-qubit unitary path. Measurement is destructive
and always in the computational (Z) basis. None of that is accidental —
it's the honest price of an exact classical simulation, and exactly why
real quantum hardware, which sidesteps the exponential memory problem
entirely, is worth building.
