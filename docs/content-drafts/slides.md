---
marp: true
theme: default
paginate: true
---

# qsim
### A parallel quantum circuit simulator in Go

github.com/Pisush/qsim

<!-- notes: Cold open. Say up front: no physics degree required, just
slices, complex arithmetic, and goroutines. Set expectations for a
code-first talk. -->

---

# Quantum computers don't exist on your laptop

- ...but their math does.
- n qubits → 2^n complex amplitudes, one per possible bit string.
- A gate is a small unitary matrix that rewrites those amplitudes.
- Simulate the math exactly, pay for it in memory and CPU instead.

<!-- notes: This is the whole premise of the project in one slide. Land
the "exact, not approximate" point — it's what distinguishes this from a
toy demo. -->

---

# The state is just a slice

```go
amps := make([]complex128, 1<<uint(nQubits))
amps[0] = 1 // |00...0>
```

- `State` wraps a `[]complex128` of length `2^n`. No wrapper type per
  qubit, no tensor-product machinery.
- Index `i`, bit `k` of `i` tells you what qubit `k` reads.
- Entanglement is just correlations across indices of one flat slice.

<!-- notes: Show NewState(2) live if doing a live demo. Emphasize there's
no "qubit object" anywhere in the type system — that surprises people. -->

---

# The honest cost

- 24 qubits = 256 MiB of amplitudes.
- 26 qubits = 1 GiB.
- `DefaultQubitCap` is 26 — `NewState` refuses to allocate past it.
- `WithQubitCap` overrides it, for when you really mean it.

<!-- notes: This cap is a guard rail, not an arbitrary limit — it exists so
an `-n 40` typo fails fast with a clear error instead of trying to allocate
16 exbibytes. -->

---

# Gates without giant matrices

- Textbook approach: build the 2^n × 2^n unitary (tensor product with
  identity for every other qubit), multiply. Mostly zeros.
- qsim's approach: a gate only ever mixes **pairs** of amplitudes whose
  indices differ in one bit.
- O(2^n) work per gate instead of O(4^n).

<!-- notes: The "mostly zeros" framing is the setup for the punchline on
the next slide — nobody would multiply by an identity matrix to leave a
number unchanged, so why build one to leave 2^n - 2 amplitudes unchanged? -->

---

# The actual code

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

<!-- notes: Walk this loop live on a 2-qubit example applying H to qubit 0.
`Gate` is a `[2][2]complex128` — genuinely just that. -->

---

# CNOT is the same trick plus a mask

- `ApplyControlled(g, control, target)` reuses the identical pair-mixing
  loop.
- One extra check: skip the pair unless the control qubit's bit is set.
- `CNOT` = `ApplyControlled(X, control, target)`. `CZ` = same with `Z`.

<!-- notes: Four lines gets you a two-qubit entangling gate. This is the
moment to mention entanglement is "free" — it's just what correlated
amplitudes look like once you stop factoring the slice per qubit. -->

---

# Why this is a parallelism gift

- Amplitude pairs for one gate are **disjoint by construction**.
- Hand a goroutine a range of pairs → it never touches another
  goroutine's data.
- No mutexes. No atomics. Just a `sync.WaitGroup`.

<!-- notes: This is the thesis of the talk: the representation that makes
gates cheap to apply is also the representation a worker pool wants for
free. Not a coincidence once you see the pair structure. -->

---

# Balancing the work: `pairIndex`

```go
func pairIndex(p, qubit int) int {
    mask := 1<<uint(qubit) - 1
    return (p&^mask)<<1 | p&mask
}
```

- Enumerates pairs by index `p`, not by contiguous block.
- Inserts a zero bit at position `qubit` into `p` to get the pair's low
  member.
- Keeps work balanced across workers for *every* qubit, including the top
  one — a naive contiguous-block split gets that case wrong.

<!-- notes: Worth a beat: for the top qubit, one contiguous half of the
slice is one giant block. A block-based split hands one goroutine
everything and the rest nothing; pairIndex avoids that. -->

---

# The worker pool

```go
chunk := (half + workers - 1) / workers
for lo := 0; lo < half; lo += chunk {
    hi := min(lo+chunk, half)
    go func(lo, hi int) {
        defer wg.Done()
        for p := lo; p < hi; p++ { /* mix pair p */ }
    }(lo, hi)
}
wg.Wait()
```

- `workers = runtime.NumCPU()`.
- Below `parallelThreshold` (2^14 amplitudes / 14 qubits): serial fallback.

<!-- notes: Goroutine startup and WaitGroup overhead dominate below the
threshold — that's an empirical cutoff backed by the benchmarks, not a
guess. -->

---

# Benchmarks (8-core Apple Silicon)

| kernel | 20q serial | 20q parallel | 22q serial | 22q parallel |
|---|---:|---:|---:|---:|
| ApplyGate | 1.82 ms | 0.36 ms | 4.2 ms | 1.7 ms |
| ApplyControlled | 1.30 ms | 0.58 ms | — | — |

- `go test -bench . ./...` reproduces this on your own machine.
- Test suite runs clean under `go test -race`.

<!-- notes: Live-run the benchmark here if doing a live demo — audiences
trust numbers they watch print more than numbers on a slide. -->

---

# Circuits, fluently

```go
c := qsim.NewCircuit(2)
c.H(0).CNOT(0, 1)

s, _ := qsim.NewState(2)
c.Run(s) // Bell state (|00> + |11>)/root 2
```

- `Circuit` is a reusable *description*: no state vector until `Run`.
- One `Circuit` can `Run` against any number of fresh `State`s.

<!-- notes: This reusability is exactly what the Grover demo needs for
sampling thousands of runs against fresh states cheaply. -->

---

# Grover's search: O(root N) instead of O(N)

- Classical search of N unsorted items: O(N) look-ups, no way around it.
- Grover's algorithm: O(root N), via repeated oracle + diffusion rounds.
- Each iteration: phase-flip the marked amplitude, then invert about the
  mean.

<!-- notes: This is the demo that makes "quantum speedup" visible instead
of asserted — the whole reason it's the flagship demo. -->

---

# One Grover iteration, three primitives

```go
func iterate(s *qsim.State, marked int) {
    s.FlipPhase(marked) // oracle
    for q := 0; q < s.NumQubits(); q++ {
        s.ApplyGate(qsim.H, q)
    }
    s.FlipPhase(0)
    for q := 0; q < s.NumQubits(); q++ {
        s.ApplyGate(qsim.H, q)
    }
}
```

<!-- notes: FlipPhase, ApplyGate(H), FlipPhase, ApplyGate(H) again. That's
the entire diffusion operator — inversion about the mean — expressed with
primitives already in the package. -->

---

# Watch the probability climb — and fall

```sh
go run ./cmd/grover -n 10 -marked 42
```

- Prints success probability after every iteration as an ASCII bar chart.
- Peaks at `floor(pi/4 * sqrt(N))` iterations.
- **Keeps iterating past the peak → probability decreases.**

<!-- notes: This is the emotional payoff of the whole talk. Narrate the
climb, pause on the optimal-iteration marker, then show a re-run with
double the iterations to prove the decline is real, not a fluke. -->

---

# Checking the theory against reality

```text
found the marked element in 998/1000 trials (99.8%);
theory predicts 99.9%
```

- After charting the curve, the tool samples 1,000 real `Measure` calls at
  the optimal iteration count.
- Empirical hit rate compared directly against
  `grover.TheoreticalProbability`.

<!-- notes: Close the loop: theory, simulate, then literally sample and
check. This is what makes it a simulator and not just a chart. -->

---

# What this is (honestly) not

- Pure states only — no noise model, no density matrices, no error
  correction.
- Single- and controlled-single-qubit gates only — no native multi-qubit
  unitaries (Toffoli needs decomposition).
- Measurement is destructive, computational (Z) basis only.

<!-- notes: None of this is accidental — it's the honest price of exact
classical simulation, and exactly why real quantum hardware, which
sidesteps the 2^n memory tax, is worth building. -->

---

# Try it yourself

```sh
go get github.com/Pisush/qsim
go test -bench . ./...
go run ./cmd/grover -n 12 -marked 7
```

- Repo: github.com/Pisush/qsim
- "Quantum circuits are just slices and goroutines wearing a trench coat."

<!-- notes: Closing line, repo link, open for Q&A. -->
