package main

import (
	"fmt"
	"math"

	"github.com/MeKo-Christian/simd-wasm/simd"
)

// checkAfterGrowth forces the Go runtime to grow its linear memory well after
// the kernel module was instantiated against it, then re-runs a kernel. The
// kernel imports the *same* WebAssembly.Memory object, so growth is shared and
// its view stays valid -- this is what makes the shared-memory design safe.
func checkAfterGrowth() bool {
	ballast := make([][]float32, 0, 64)
	for range 64 {
		ballast = append(ballast, make([]float32, 1<<18)) // ~64 MiB total
	}

	a := ballast[len(ballast)-1]
	for i := range a {
		a[i] = 1
	}
	got, want := simd.Dot(a, a), simd.ScalarDot(a, a)
	ok := math.Abs(float64(got-want)) <= float64(want)*1e-5
	fmt.Printf("%s Dot after ~64MiB memory growth: %v ~= %v\n", status(ok), got, want)

	return ok
}

func status(ok bool) string {
	if ok {
		return "ok  "
	}

	return "FAIL"
}
