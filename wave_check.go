package main

import (
	"fmt"
	"math"

	"github.com/MeKo-Christian/simd-wasm/sim"
)

// checkWave steps the same pond on all three backends and asserts they agree.
// The kernels only exist inside a wasm build, so this -- not `go test` -- is
// where a wrong stencil gets caught, and it runs in CI through `just run` and
// `just run-js`.
func checkWave() bool {
	const (
		w, h  = 96, 64
		steps = 200
		// The three backends sum the Laplacian's four neighbours in the same
		// order, but clang contracts the multiply-add differently from Go, so
		// 200 compounding steps drift slightly rather than matching bit for bit.
		tol = 1e-4
	)

	fields := make([][]float32, 0, len(sim.Backends))

	for _, b := range sim.Backends {
		s := sim.NewWave(w, h)
		s.Poke(w/3, h/2, 1)
		s.Poke(2*w/3, h/3, -1)
		s.Step(b, steps)
		fields = append(fields, append([]float32(nil), s.Field()...))
	}

	ok := true

	for i := 1; i < len(fields); i++ {
		worst, at := 0.0, -1

		for j := range fields[i] {
			d := math.Abs(float64(fields[i][j] - fields[0][j]))
			if d > worst {
				worst, at = d, j
			}
		}

		if worst > tol {
			fmt.Printf("FAIL Wave %s vs %s: max delta %v at cell %d\n",
				sim.Backends[i], sim.Backends[0], worst, at)

			ok = false

			continue
		}

		fmt.Printf("%s Wave %-8s matches %s after %d steps (max delta %.2e)\n",
			status(true), sim.Backends[i].String(), sim.Backends[0], steps, worst)
	}

	return ok
}
