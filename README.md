# qsim

A state-vector quantum circuit simulator in Go, with parallel gate application
and a Grover's-algorithm demo.

[![Go Reference](https://pkg.go.dev/badge/github.com/Pisush/qsim.svg)](https://pkg.go.dev/github.com/Pisush/qsim)

## The physics, in ~200 words

A classical bit is either 0 or 1. A **qubit** can be in a *superposition*: a
blend of both at once, described by two complex numbers (amplitudes) whose
squared magnitudes give the probability of measuring 0 or 1. A system of *n*
qubits needs 2^n amplitudes — one for every possible bit string — which is why
simulating even 30 qubits strains a laptop, and why quantum hardware is
interesting.

A **quantum gate** is a small unitary matrix that rewrites amplitudes. Some
gates act on one qubit (like `H`, which turns a definite 0 into an equal mix
of 0 and 1); others, like `CNOT`, act on two and create **entanglement** —
correlations between qubits that have no classical counterpart. A quantum
program ("circuit") is just a sequence of gates.

**Measurement** is where probability enters: reading a qubit forces it to pick
0 or 1 at random, weighted by its amplitudes, and the state *collapses* to
agree with the result.

This simulator stores all 2^n amplitudes in a `[]complex128` and applies gates
by mixing pairs of amplitudes directly — no giant matrices — so circuits up to
~24 qubits run comfortably on a laptop.

## Install

```sh
go get github.com/Pisush/qsim
```

## Usage

### Bell state, step by step

```go
s, err := qsim.NewState(2) // |00⟩
if err != nil {
    log.Fatal(err)
}
s.ApplyGate(qsim.H, 0)         // qubit 0 into superposition
s.ApplyControlled(qsim.X, 0, 1) // CNOT entangles qubit 1 with qubit 0

rng := rand.New(rand.NewSource(42)) // all randomness is injected
bits := s.MeasureAll(rng)           // always [0 0] or [1 1] — never mixed
```

### The same circuit, fluently

```go
c := qsim.NewCircuit(2)
c.H(0).CNOT(0, 1)

s, _ := qsim.NewState(2)
c.Run(s) // circuits are reusable descriptions; Run them on any fresh state
```

Available gates: `X, Y, Z, H, S, T` plus parameterized `Rx(θ), Ry(θ), Rz(θ),
Phase(θ)`, each with a matching `Circuit` method, and `Controlled`/`CNOT`/`CZ`
for two-qubit operations. `FlipPhase(i)` negates one basis amplitude — the
oracle primitive used by Grover's search.

### Grover's search

```sh
go run ./cmd/grover -n 10 -marked 42
```

prints the success probability after each Grover iteration — rising to ~99.9%
at `floor(π/4·√N)` iterations and falling if you overshoot — then verifies the
peak by sampling measurements. The `grover` package exposes the same as a
library (`grover.Curve`, `grover.Search`).

## Concurrency

Gate application is parallelized: the 2^n/2 amplitude pairs are split evenly
across `runtime.NumCPU()` goroutines, each pair owned by exactly one worker.
Below 2^14 amplitudes a serial path is used instead, where goroutine overhead
would dominate. On an 8-core Apple Silicon machine (`go test -bench .`):

| kernel          | 20 qubits serial | 20 qubits parallel | 22 qubits serial | 22 qubits parallel |
|-----------------|-----------------:|-------------------:|-----------------:|-------------------:|
| ApplyGate       | 1.82 ms          | 0.36 ms            | 4.2 ms           | 1.7 ms             |
| ApplyControlled | 1.30 ms          | 0.58 ms            | —                | —                  |

A single `State` is not safe for concurrent use, but independent `State`
values may be simulated concurrently — the package has no global mutable
state, and the test suite runs clean under `-race`.

## Known limitations

- **State-vector simulation only.** Memory doubles per qubit: 24 qubits =
  256 MiB of amplitudes, 26 = 1 GiB. `NewState` refuses to allocate above a
  cap (default 26 qubits, adjustable via `WithQubitCap`).
- **Pure states only.** No noise model, no density matrices, no error
  correction.
- **Single- and controlled-single-qubit gates only.** No native multi-qubit
  unitaries (Toffoli must be decomposed, or use `FlipPhase`-style diagonal
  tricks).
- Measurement is destructive and in the computational (Z) basis only; rotate
  first to measure in another basis.
