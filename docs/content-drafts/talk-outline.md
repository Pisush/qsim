> DRAFT — conference talk outline for qsim (github.com/Pisush/qsim)

# Talk outline: Building a parallel quantum simulator in Go

**Format:** 25-30 min conference talk (e.g. GopherCon-style track talk)
**Audience:** Go engineers with no assumed quantum computing background;
comfortable with goroutines, slices, and benchmarking.

## CFP abstract (150-200 words)

Quantum computers don't exist on your laptop, but their math does. Any
quantum circuit can be simulated exactly by storing 2^n complex numbers —
one per possible n-qubit basis state — and rewriting that vector one gate
at a time. The catch is the exponential: every qubit you add doubles the
memory and the work.

This talk builds a state-vector quantum simulator live, in Go, using
qsim as the running example. We'll see why gates never need a full
2^n × 2^n matrix — just pairwise amplitude mixing — and why that
representation happens to be exactly what a goroutine worker pool wants:
disjoint amplitude pairs, no shared state, no locks. We'll benchmark
serial versus parallel gate application at 20+ qubits and watch the
crossover where spinning up workers stops being worth it.

We'll close with Grover's search — the algorithm that finds a marked
item in O(√N) instead of O(N) — running on the simulator, watching its
success probability climb to a peak and then *fall* if you keep
iterating past it. No physics degree required; just amplitudes, slices,
and goroutines.

## Section breakdown with timings

**1. Cold open — what a qubit even is (3 min)**
- A classical bit is 0 or 1. A qubit is a blend of both, described by two
  complex amplitudes whose squared magnitudes are probabilities.
- n qubits need 2^n amplitudes — the reason simulating 30 qubits strains
  a laptop and quantum hardware is interesting at all.
- Frame the talk: we're going to build the "boring" (classical, exact)
  side of this, in Go, and it's more approachable than it sounds.

**2. The state is a slice (4 min)**
- `[]complex128` of length `1<<n`; index `i`, bit `k` of `i` is what
  qubit `k` reads. Live: `qsim.NewState(2)` gives `|00⟩`.
- No wrapper type per qubit, no tensor-product machinery — entanglement
  is just correlations across indices of one flat slice.
- Mention the honest cost: 24 qubits = 256 MiB, 26 = 1 GiB, and qsim's
  `DefaultQubitCap` refuses to let you allocate past that by accident.

**3. Gates without giant matrices (6 min)**
- Show the textbook 2^n × 2^n tensor-product gate matrix (mostly zeros)
  next to qsim's actual `applyGateSerial`: pair up amplitudes differing
  in one bit, mix by a 2×2 matrix.
- Walk the `stride` / `base` loop on the whiteboard/slide for a 2-qubit
  example applying `H` to qubit 0 — show it by hand matching the Bell
  state result.
- `ApplyControlled` as the same trick plus one mask check for the
  control bit — CNOT in four lines.

**4. Why this is a parallelism gift (7 min)**
- The key insight: amplitude pairs for one gate are *disjoint by
  construction*. Hand a goroutine a range of pairs and it never touches
  another goroutine's data. No mutexes, no atomics.
- Show `pairIndex`'s bit trick for balanced work at any qubit position,
  including the top qubit (the case a naive contiguous-block split gets
  wrong).
- Live benchmark: `go test -bench .` on the actual repo, serial vs.
  parallel `ApplyGate` at 20 and 22 qubits. Point out the
  `parallelThreshold` (2^14 amplitudes) and why small states go serial —
  goroutine overhead dominates below it.
- Land the one design rule that keeps this safe: a `State` isn't
  concurrency-safe *by itself*, but independent `State`s have zero
  shared global state, so many simulations run concurrently for free.
  `-race` stays clean.

**5. Live demo: Grover's search (7 min — see demo plan below)**

**6. What this is not, and why that's fine (2 min)**
- Pure states only, no noise, no density matrices, no error correction;
  single- and controlled-single-qubit gates only (Toffoli needs
  decomposition).
- This is the honest cost of *exact* classical simulation — and exactly
  why real hardware, which doesn't pay the 2^n memory tax, matters.

**7. Close / Q&A (1-2 min)**
- Repo link, one-line pitch: "quantum circuits are just slices and
  goroutines wearing a trench coat."

## Live-demo plan

**Setup (before the talk):** repo cloned and built locally;
`go test -bench . -benchtime 3x` pre-warmed once so the first live run
isn't paying cold-cache/JIT-adjacent variance; terminal font large;
`cmd/grover` and `bench_test.go` both open in tabs.

**Demo 1 — benchmarks (during section 4, ~2 min):**
```sh
go test -bench BenchmarkApplyGate -benchtime 2s ./...
```
Point at the serial vs. parallel numbers as they print; compare live
against the table already in the README so the audience sees it's not
cherry-picked.

**Demo 2 — Grover's search (section 5, ~7 min):**
```sh
go run ./cmd/grover -n 10 -marked 42
```
- Narrate the printed iteration-by-iteration ASCII bar chart as it
  scrolls: probability climbing from ~0.1% toward the peak near
  `floor(π/4·√1024) ≈ 25` iterations, then declining past it — pause on
  the `<- optimal` and `<- peak` markers the tool prints.
- Re-run with `-iters` doubled to show the decline continuing past the
  peak, driving home "more iterations ≠ better."
- Finish on the empirical check: 1,000 sampled measurements at the
  optimal iteration count, hit rate compared live against the
  theoretical prediction the tool prints alongside it.
- Fallback if live coding/networking misbehaves: a pre-recorded terminal
  session (asciinema) of the same three commands, narrated identically.

**Time budget safety valve:** if running short on time, cut Demo 1 to a
single pre-run screenshot and keep Demo 2 live in full — Grover's rise-
and-fall curve is the talk's emotional payoff and should never be cut.
