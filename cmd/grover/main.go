// Command grover demonstrates Grover's search on the qsim simulator.
//
// It searches N = 2^n elements for one marked element, prints the
// success probability after each Grover iteration (showing the
// characteristic rise to a peak at floor(pi/4*sqrt(N)) and the decline
// past it), and then verifies the peak empirically by sampling
// measurements.
//
// Usage:
//
//	grover [-n qubits] [-marked index] [-iters count] [-trials count] [-seed seed]
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/Pisush/qsim/grover"
)

func main() {
	var (
		nQubits = flag.Int("n", 10, "number of qubits (search space is 2^n elements)")
		marked  = flag.Int("marked", -1, "marked element to search for (-1: pick one at random)")
		iters   = flag.Int("iters", -1, "iterations to chart (-1: twice the optimal count)")
		trials  = flag.Int("trials", 1000, "measurement trials for the empirical check")
		seed    = flag.Int64("seed", 1, "random seed for marking and measurement")
	)
	flag.Parse()

	rng := rand.New(rand.NewSource(*seed))
	n := 1 << uint(*nQubits)
	if *marked < 0 {
		*marked = rng.Intn(n)
	}
	kOpt := grover.OptimalIterations(*nQubits)
	if *iters < 0 {
		*iters = 2 * kOpt
	}

	fmt.Printf("Grover search: N = 2^%d = %d elements, marked = %d\n", *nQubits, n, *marked)
	fmt.Printf("optimal iterations: floor(pi/4*sqrt(N)) = %d\n\n", kOpt)

	probs, err := grover.Curve(*nQubits, *marked, *iters)
	if err != nil {
		fmt.Fprintln(os.Stderr, "grover:", err)
		os.Exit(1)
	}

	fmt.Println("iteration  P(marked)  ")
	peak := 0
	for k, p := range probs {
		if p > probs[peak] {
			peak = k
		}
	}
	for k, p := range probs {
		bar := strings.Repeat("#", int(p*40+0.5))
		note := ""
		if k == kOpt {
			note = "  <- optimal"
		}
		if k == peak && peak != kOpt {
			note += "  <- peak"
		}
		fmt.Printf("%9d  %9.6f  %-40s%s\n", k, p, bar, note)
	}

	fmt.Printf("\nsampling %d runs at %d iterations...\n", *trials, kOpt)
	hits := 0
	for i := 0; i < *trials; i++ {
		got, _, err := grover.Search(*nQubits, *marked, rng)
		if err != nil {
			fmt.Fprintln(os.Stderr, "grover:", err)
			os.Exit(1)
		}
		if got == *marked {
			hits++
		}
	}
	fmt.Printf("found the marked element in %d/%d trials (%.1f%%); theory predicts %.1f%%\n",
		hits, *trials, 100*float64(hits)/float64(*trials),
		100*grover.TheoreticalProbability(*nQubits, kOpt))
}
