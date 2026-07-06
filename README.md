# qsim

A state-vector quantum circuit simulator in Go, with parallel gate application
and a Grover's-algorithm demo.

## The physics, in ~200 words

A classical bit is either 0 or 1. A **qubit** can be in a *superposition*: a
blend of both at once, described by two complex numbers (amplitudes) whose
squared magnitudes give the probability of measuring 0 or 1. A system of *n*
qubits needs 2^n amplitudes — one for every possible bit string — which is why
simulating even 30 qubits strains a laptop, and why quantum hardware is
interesting.

A **quantum gate** is a small unitary matrix that rewrites amplitudes. Some
gates act on one qubit (like `H`, which turns a definite 0 into an equal
mix of 0 and 1); others, like `CNOT`, act on two and create
**entanglement** — correlations between qubits that have no classical
counterpart. A quantum program ("circuit") is just a sequence of gates.

**Measurement** is where probability enters: reading a qubit forces it to pick
0 or 1 at random, weighted by its amplitudes, and the state *collapses* to
agree with the result.

This simulator stores all 2^n amplitudes in a `[]complex128` and applies gates
by mixing pairs of amplitudes directly — no giant matrices — so circuits up to
~24 qubits run comfortably on a laptop.

## Status

Under construction — see open pull requests for progress.

## Known limitations

- State-vector simulation only: memory doubles per qubit (default cap: 26
  qubits = 1 GiB of amplitudes).
- No noise model, no density matrices — pure states only.
