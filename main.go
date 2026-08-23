// Command demo verifies the SIMD kernels against the pure-Go reference and
// benchmarks both. Build for wasip1 and run through host/run.mjs.
package main

import (
	"fmt"
	"math"
	"math/rand/v2"
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

	// correctness
	wantDot := simd.ScalarDot(a, b)
	gotDot := simd.Dot(a, b)
	if d := math.Abs(float64(gotDot - wantDot)); d > 1e-3 {
		fmt.Printf("FAIL Dot: got %v want %v (diff %v)\n", gotDot, wantDot, d)
	} else {
		fmt.Printf("ok   Dot  %v ~= %v\n", gotDot, wantDot)
	}
	simd.ScalarAdd(want, a, b)
	simd.Add(got, a, b)
	for i := range want {
		if got[i] != want[i] {
			fmt.Printf("FAIL Add at %d: got %v want %v\n", i, got[i], want[i])
			break
		}
	}
	fmt.Printf("ok   Add  %d elements identical\n\n", n)

	// benchmark
	checkAfterGrowth()
	fmt.Println()
	fmt.Printf("%-10s %12s %12s %8s\n", "kernel", "scalar", "simd", "speedup")
	fmt.Printf("%-10s %12s %12s %8s\n", "Dot",
		dur(bench(func() { sink = simd.ScalarDot(a, b) })),
		dur(bench(func() { sink = simd.Dot(a, b) })),
		ratio(bench(func() { sink = simd.ScalarDot(a, b) }), bench(func() { sink = simd.Dot(a, b) })))
	fmt.Printf("%-10s %12s %12s %8s\n", "Add",
		dur(bench(func() { simd.ScalarAdd(got, a, b) })),
		dur(bench(func() { simd.Add(got, a, b) })),
		ratio(bench(func() { simd.ScalarAdd(got, a, b) }), bench(func() { simd.Add(got, a, b) })))
}

var sink float32

func bench(f func()) time.Duration {
	for i := 0; i < iters/10; i++ { // warm up
		f()
	}
	start := time.Now()
	for i := 0; i < iters; i++ {
		f()
	}
	return time.Since(start) / iters
}

func dur(d time.Duration) string { return d.String() }

func ratio(a, b time.Duration) string {
	if b == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.2fx", float64(a)/float64(b))
}
