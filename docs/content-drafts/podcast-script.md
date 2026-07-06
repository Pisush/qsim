DRAFT — podcast script for qsim (github.com/Pisush/qsim)

# Podcast script: "Slices, Not Matrices" — building a quantum simulator in Go

*Two hosts. HOST A is a curious generalist — comfortable with Go, no quantum
background. HOST B is the builder of qsim and knows the codebase cold.
Target runtime: ~15-18 minutes.*

---

**HOST A:** Okay, so today we're talking about quantum computing, but not the
scary version — no dilution refrigerators, no physics PhD required. You built
a quantum circuit simulator. In Go. Called qsim. Before we get into the code,
give me the one-sentence pitch.

**HOST B:** A quantum computer with *n* qubits needs 2^n complex numbers to
describe its state completely — one for every possible string of n bits.
qsim just... stores those numbers in a slice, `[]complex128`, and rewrites
them when you apply a gate. That's the whole simulator. No quantum hardware
involved, just exact math running on your laptop.

**HOST A:** "Exact math" is doing some work there. Exact meaning what,
exactly?

**HOST B:** Meaning it's not an approximation. If you had infinite memory,
you could simulate any quantum circuit perfectly this way — the catch is
that 2^n grows so fast it isn't infinite memory you need, it's *impossible*
memory. 24 qubits is 256 mebibytes of amplitudes. 26 qubits is a full
gibibyte. qsim actually has a hard cap — `DefaultQubitCap` is 26 — so if you
typo `-n 40` it fails fast with a clear error instead of trying to allocate
16 exbibytes and thrashing your machine into the ground.

**HOST A:** So that's the ceiling, and it's the entire reason real quantum
hardware exists — it doesn't pay that 2^n tax.

**HOST B:** Exactly. Every simulator like this is secretly an advertisement
for why quantum hardware matters. But below that ceiling, it's a really fun
sandbox.

**HOST A:** Let's get into the one clever idea, because I read the source
before this and there's no giant matrix anywhere. I expected a `2^n x 2^n`
matrix multiply for every gate. Where did it go?

**HOST B:** That's the whole trick, and once you see it you can't unsee it.
The textbook description of a quantum gate is a unitary matrix, and to apply
a gate to one qubit out of n, you tensor it with identity matrices for every
other qubit, and you get this enormous matrix that's almost entirely zeros.
Then you multiply the state vector by that giant sparse thing. It's
correct, but it's throwing away information you already had: the gate is
only ever a 2-by-2 matrix. All the tensor-product machinery does is figure
out *which pairs of amplitudes* that 2-by-2 matrix should act on.

**HOST A:** So you just... figure out the pairs directly.

**HOST B:** Right. For a gate on qubit `q`, the two amplitudes it mixes are
the ones whose indices differ in exactly bit `q`. So you walk the slice with
a stride of `1 << q`, and for every pair `(amps[i], amps[i+stride])`, you
replace them with the gate matrix applied to that 2-vector. Here's the
literal serial version:

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

**HOST A:** That's it? That's the quantum gate?

**HOST B:** That's it. `Gate` itself is just a `[2][2]complex128`. Hadamard,
Pauli-X, Pauli-Y, Pauli-Z, S, T, the parameterized rotations — they're all a
few lines of complex arithmetic defining one of these 2-by-2 matrices.
`ApplyControlled`, which gives you CNOT and CZ, is the identical loop with
one extra check: skip the pair unless the control qubit's bit is set.

**HOST A:** Okay, here's the part I actually wanted to talk to you about,
because this is where Go stops being an implementation detail and starts
being the reason the project exists. Why goroutines? Why does this problem
want concurrency?

**HOST B:** Because look at that loop again — every pair `(i, i+stride)` is
completely independent of every other pair. Gate application doesn't read or
write anything outside its own pair. That's the dream scenario for
parallelism: no shared mutable state, no coordination needed *during* the
work, just partition the pairs and hand each partition to a goroutine.

**HOST A:** No mutexes.

**HOST B:** No mutexes, no atomics. `parallel.go` splits the `len(amps)/2`
pairs across `runtime.NumCPU()` goroutines, each goroutine owns its range
outright, and you just `sync.WaitGroup.Wait()` at the end. There's one bit
trick worth mentioning — `pairIndex` — because you can't just chop the slice
into contiguous blocks; for some qubit positions that gives one goroutine
almost the whole state vector and everyone else nothing. `pairIndex` inserts
a zero bit at position `qubit` into a running counter `p`, so pairs are
enumerated in a way that stays balanced no matter which qubit you're gating.

**HOST A:** And there's a cutoff where you don't bother.

**HOST B:** Right, `parallelThreshold` is `1 << 14` — 14 qubits worth of
amplitudes. Below that, spinning up goroutines and waiting on a WaitGroup
costs more than the arithmetic itself would take serially, so `ApplyGate`
just runs the plain loop. The README has real benchmarks — at 20 qubits,
`ApplyGate` goes from about 1.82 milliseconds serial to 0.36 milliseconds
parallel on an 8-core machine. At 22 qubits it's 4.2 down to 1.7.
`ApplyControlled` shows the same shape, just a smaller margin, because
roughly half its pairs get skipped by the control-bit check no matter how
many workers you throw at it.

**HOST A:** Is a `State` safe to hand to multiple goroutines at once, then?
Given everything you just said about disjoint pairs, I could see someone
assuming yes.

**HOST B:** Good instinct to check, and the answer is no, deliberately. A
single `*State` is not safe for concurrent use — if you called `ApplyGate`
on the same state from two goroutines at once, you'd have two worker pools
racing over the same slice. What *is* true is that the package keeps zero
global mutable state, so two completely independent `State` values can run
on different goroutines with no coordination at all. If you want to run a
thousand independent simulations to build up statistics — which is exactly
what the Grover demo's sampling step does — you just fire off a thousand
goroutines, each with its own `*State`, and there's nothing to protect. The
test suite runs clean under `go test -race`, which is the actual proof, not
just a claim.

**HOST A:** You mentioned a circuit builder earlier in passing — talk me
through that, because raw `ApplyGate` calls on a `*State` sound like they'd
get verbose for anything beyond a two-qubit toy.

**HOST B:** They do, fast. So `circuit.go` adds `Circuit`, which is just an
ordered list of gate operations — a gate, a target qubit, and a control
qubit if it's a controlled operation — built fluently:

```go
c := qsim.NewCircuit(2)
c.H(0).CNOT(0, 1)

s, _ := qsim.NewState(2)
c.Run(s) // s is now the Bell state (|00> + |11>)/root 2
```

**HOST A:** And a `Circuit` doesn't hold a state vector itself.

**HOST B:** Right, that's the important design choice — a `Circuit` is a
description, not a computation. It allocates no amplitudes at all. That
means one `Circuit` can `Run` against any number of fresh `State` values,
which is precisely what you want for something like Grover's sampling step:
build the circuit once, then run it a thousand times against a thousand
fresh states to gather statistics, without re-describing the gate sequence
each time.

**HOST A:** Let's do the "let me run the demo" bit. What do I actually see
if I run this thing?

**HOST B:** Let's do Grover's search, because it's the demo that makes the
speedup *visible* instead of asserted. Classically, if you're searching N
unsorted items for one marked item, you need O(N) look-ups on average — no
way around it, you have to check things. Grover's algorithm does it in
O(root N) using superposition and something called amplitude amplification.
The command is `go run ./cmd/grover -n 10 -marked 42` — that's a search
space of 2^10, 1024 elements, and we're looking for index 42.

**HOST A:** And what prints?

**HOST B:** It prints the probability of measuring the marked element after
each Grover iteration — 0, 1, 2, 3... and you watch that number climb from
under a tenth of a percent up toward basically 100%, peaking at
`floor(pi/4 * sqrt(N))` iterations, which for N=1024 is 25. And then — this
is the detail that gets people — if you keep iterating past 25, the
probability doesn't stay high. It goes back *down*. Overshoot the peak and
you're worse off than you were a few iterations earlier.

**HOST A:** Why does it fall instead of just plateauing?

**HOST B:** Because each Grover iteration is a rotation — literally, in the
two-dimensional subspace spanned by the marked state and everything else,
one iteration rotates your state vector by a fixed angle toward the marked
state. Keep rotating past the point where you're aligned with it, and you
rotate *past* it, back toward the unmarked subspace. The closed form is
`sin squared of (2k+1) theta`, and `sin squared` goes up and comes back down
— that's just what sine does.

**HOST A:** And under the hood, what's actually running each iteration?

**HOST B:** Three lines, and they're all primitives we already talked about.
`FlipPhase(marked)` is the oracle — it just negates the amplitude at the
marked index, no gate matrix needed, it's a single slice write. Then
Hadamard on every qubit, `FlipPhase(0)`, Hadamard on every qubit again — that
sequence realizes the diffusion operator, which is "inversion about the
mean." That's genuinely the whole iteration.

**HOST A:** And then after charting the climb, the tool double-checks
itself?

**HOST B:** Yeah — after printing the curve, it samples a thousand actual
measurements at the optimal iteration count and reports the empirical hit
rate next to the theoretical prediction, so you're not just trusting the
math, you're watching the simulator's own `Measure` calls agree with it.

**HOST A:** That's a nice closing loop — theory, simulate, then literally
sample and check.

**HOST B:** That's honestly most of the fun of this project. You get to
watch an algorithm that sounds like magic turn out to be a rotation you can
graph with ASCII bars in a terminal.

**HOST A:** Where should people start if they want to poke at this
themselves?

**HOST B:** Clone it, run `go test -bench . ./...` to see the serial-versus-
parallel numbers on your own machine, then run the Grover command with a
bigger `-n` and watch the peak iteration count grow with `sqrt(N)`. It caps
out around 24-26 qubits depending on your RAM, and honestly hitting that
ceiling is part of the point.

**HOST A:** Slices, not matrices. I like it. Thanks for building this.

**HOST B:** Thanks for having me.

---

*Approx. 1,950 words.*
