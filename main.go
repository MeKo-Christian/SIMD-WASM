// Command demo verifies the SIMD kernels against the pure-Go reference and
// benchmarks both. Build for wasip1 or js and run through host/run.mjs or
// host/run-js.mjs. It exits non-zero if any check fails, so CI can gate on it.
package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"time"

	"github.com/MeKo-Christian/simd-wasm/simd"
)

const (
	n     = 4096
	iters = 2000
)

func main() {
	fmt.Printf("SIMD kernels linked: %v\n\n", simd.Enabled())

	a := make([]float32, n)
	b := make([]float32, n)

	for i := range a {
		a[i] = rand.Float32()*2 - 1
		b[i] = rand.Float32()*2 - 1
	}
	got := make([]float32, n)
	want := make([]float32, n)

	ok := true

	// correctness: Dot accumulates in a different order than the scalar loop,
	// so compare within a tolerance rather than exactly.
	wantDot, gotDot := simd.ScalarDot(a, b), simd.Dot(a, b)
	dotOK := math.Abs(float64(gotDot-wantDot)) <= 1e-3
	ok = ok && dotOK
	fmt.Printf("%s Dot  %v ~= %v\n", status(dotOK), gotDot, wantDot)

	// Add is elementwise, so it must match bit for bit.
	simd.ScalarAdd(want, a, b)
	simd.Add(got, a, b)
	addOK := true

	for i := range want {
		if got[i] != want[i] {
			fmt.Printf("FAIL Add at %d: got %v want %v\n", i, got[i], want[i])
			addOK = false
			break
		}
	}

	ok = ok && addOK
	if addOK {
		fmt.Printf("ok   Add  %d elements identical\n", n)
	}

	fmt.Println()

	ok = checkAfterGrowth() && ok

	fmt.Println()

	fmt.Printf("%-10s %12s %12s %8s\n", "kernel", "scalar", "simd", "speedup")
	report("Dot", func() { sink = simd.ScalarDot(a, b) }, func() { sink = simd.Dot(a, b) })
	report("Add", func() { simd.ScalarAdd(got, a, b) }, func() { simd.Add(got, a, b) })

	if !ok {
		fmt.Fprintln(os.Stderr, "\nverification failed")
		os.Exit(1)
	}
}

// sink is written by the benchmarked closures so the compiler cannot discard
// the work being measured.
//
//nolint:unused // assigned, never read -- that is the point
var sink float32

func report(name string, scalar, vector func()) {
	s, v := bench(scalar), bench(vector)

	speedup := "n/a"
	if v > 0 {
		speedup = fmt.Sprintf("%.2fx", float64(s)/float64(v))
	}

	fmt.Printf("%-10s %12s %12s %8s\n", name, s, v, speedup)
}

func bench(f func()) time.Duration {
	for range iters / 10 { // warm up
		f()
	}
	start := time.Now()

	for range iters {
		f()
	}

	return time.Since(start) / iters
}
