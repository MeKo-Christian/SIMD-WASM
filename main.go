// Command demo verifies the compute kernels against the pure-Go reference and
// benchmarks all three backends. Build for wasip1 or js and run through
// host/run.mjs or host/run-js.mjs. It exits non-zero if any check fails, so CI
// can gate on it.
//
// Given -interactive -- which only host/index.html passes -- it instead stays
// resident and drives the browser demo. The checks run either way, so a wrong
// kernel fails loudly rather than rendering a subtly wrong ripple.
package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"slices"
	"time"

	"github.com/MeKo-Christian/simd-wasm/sim"
	"github.com/MeKo-Christian/simd-wasm/simd"
)

const (
	n = 4096

	// The benchmark grid is big enough that one step dwarfs the trampoline
	// crossing into the kernel module, and small enough to stay interactive.
	waveW, waveH = 512, 512
)

func main() {
	interactive := wantsInteractive()
	if interactive && !interactiveSupported {
		fmt.Fprintln(os.Stderr, "-interactive requires the GOOS=js build")
		os.Exit(2)
	}

	fmt.Printf("SIMD kernels linked: %v\n\n", simd.Enabled())

	if !runChecks() {
		fmt.Fprintln(os.Stderr, "\nverification failed")
		os.Exit(1)
	}

	if interactive {
		// The page runs the benchmarks on demand instead, so page load is not
		// held up by several seconds of timing loops.
		serveInteractive() // blocks

		return
	}

	fmt.Println()
	benchmarks()
}

// runChecks reports whether every kernel agrees with the pure-Go reference.
func runChecks() bool {
	a := make([]float32, n)
	b := make([]float32, n)

	for i := range a {
		a[i] = rand.Float32()*2 - 1
		b[i] = rand.Float32()*2 - 1
	}

	got := make([]float32, n)
	want := make([]float32, n)

	// correctness: Dot accumulates in a different order than the scalar loop,
	// so compare within a tolerance rather than exactly.
	wantDot, gotDot := simd.ScalarDot(a, b), simd.Dot(a, b)
	dotOK := math.Abs(float64(gotDot-wantDot)) <= 1e-3
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

	if addOK {
		fmt.Printf("ok   Add  %d elements identical\n", n)
	}

	fmt.Println()

	growOK := checkAfterGrowth()

	fmt.Println()

	waveOK := checkWave()

	return dotOK && addOK && growOK && waveOK
}

// sink is written by the benchmarked closures so the compiler cannot discard
// the work being measured.
//
//nolint:unused // assigned, never read -- that is the point
var sink float32

// benchmarks times every kernel on all three backends. Reporting the
// clang-scalar column alongside Go's is what separates the two effects the
// headline number otherwise conflates: "vs C" is the SIMD win, "vs Go" also
// counts clang's wasm codegen beating Go's.
func benchmarks() {
	a := make([]float32, n)
	b := make([]float32, n)
	scratch := make([]float32, n)

	for i := range a {
		a[i] = rand.Float32()*2 - 1
		b[i] = rand.Float32()*2 - 1
	}

	fmt.Printf("%-14s %12s %12s %12s %8s %8s\n",
		"kernel", "Go", "C", "C + SIMD", "vs Go", "vs C")

	report("Dot",
		func() { sink = simd.ScalarDot(a, b) },
		func() { sink = simd.CDot(a, b) },
		func() { sink = simd.Dot(a, b) })
	report("Add",
		func() { simd.ScalarAdd(scratch, a, b) },
		func() { simd.CAdd(scratch, a, b) },
		func() { simd.Add(scratch, a, b) })

	// One pond per backend, so no backend inherits another's cache state.
	wave := make([]*sim.Wave, len(sim.Backends()))
	for i := range wave {
		wave[i] = sim.NewWave(waveW, waveH)
		wave[i].Poke(waveW/2, waveH/2, 1)
	}

	report(fmt.Sprintf("Wave %dx%d", waveW, waveH),
		func() { wave[0].Step(sim.BackendGo, 1) },
		func() { wave[1].Step(sim.BackendC, 1) },
		func() { wave[2].Step(sim.BackendSIMD, 1) })
}

func report(name string, goScalar, cScalar, vector func()) {
	g, c, v := bench(goScalar), bench(cScalar), bench(vector)

	fmt.Printf("%-14s %12s %12s %12s %8s %8s\n",
		name, g, c, v, ratio(g, v), ratio(c, v))
}

func ratio(slow, fast time.Duration) string {
	if fast <= 0 {
		return "n/a"
	}

	return fmt.Sprintf("%.2fx", float64(slow)/float64(fast))
}

// bench runs f until it has spent roughly benchBudget on it, so a kernel that
// is a thousand times cheaper than another still gets enough repetitions to
// time, and an expensive one does not stall the page for a minute.
func bench(f func()) time.Duration {
	const (
		minIters    = 25
		maxIters    = 5000
		benchBudget = 250 * time.Millisecond
	)

	for range minIters { // warm up
		f()
	}

	start := time.Now()
	iters := 0

	for iters < maxIters {
		f()

		iters++
		if iters >= minIters && time.Since(start) >= benchBudget {
			break
		}
	}

	return time.Since(start) / time.Duration(iters)
}

func wantsInteractive() bool {
	return slices.Contains(os.Args[1:], "-interactive")
}
