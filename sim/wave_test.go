package sim_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/MeKo-Christian/simd-wasm/sim"
	"github.com/MeKo-Christian/simd-wasm/simd"
)

// referenceWaveStep is the stencil written the slowest, most obvious way, with
// plain 2D indexing and no slice aliasing. simd.ScalarWaveStep must match it
// exactly, which is what pins the optimised form down under refactoring.
func referenceWaveStep(next, cur, prev []float32, w, h int, c2, damp float32) {
	at := func(s []float32, x, y int) float32 { return s[y*w+x] }

	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			mid := at(cur, x, y)
			lap := at(cur, x-1, y) + at(cur, x+1, y) + at(cur, x, y-1) + at(cur, x, y+1) - 4*mid
			next[y*w+x] = damp * (2*mid - at(prev, x, y) + c2*lap)
		}
	}
}

func TestScalarWaveStepMatchesReference(t *testing.T) {
	t.Parallel()

	const w, h = 17, 13

	rng := rand.New(rand.NewPCG(1, 2))
	cur := make([]float32, w*h)
	prev := make([]float32, w*h)

	for i := range cur {
		cur[i] = rng.Float32()*2 - 1
		prev[i] = rng.Float32()*2 - 1
	}

	got := make([]float32, w*h)
	want := make([]float32, w*h)
	simd.ScalarWaveStep(got, cur, prev, w, h, 0.4, 0.999)
	referenceWaveStep(want, cur, prev, w, h, 0.4, 0.999)

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cell %d (x=%d, y=%d): got %v, want %v", i, i%w, i/w, got[i], want[i])
		}
	}
}

func TestWaveBorderStaysZero(t *testing.T) {
	t.Parallel()

	s := sim.NewWave(24, 20)
	s.Poke(12, 10, 1)
	s.Step(sim.BackendGo, 40)

	f := s.Field()
	for y := range s.H {
		for x := range s.W {
			if x != 0 && y != 0 && x != s.W-1 && y != s.H-1 {
				continue
			}

			if f[y*s.W+x] != 0 {
				t.Fatalf("border cell (%d, %d) is %v, want 0", x, y, f[y*s.W+x])
			}
		}
	}
}

// A stone dropped in the middle of a square pond must ripple out
// symmetrically: the stencil, the boundaries and the buffer rotation are all
// wrong in asymmetric ways if it does not. The comparison carries a tolerance
// because mirrored cells sum the same four neighbours in a different order,
// which float32 does not round identically.
func TestWaveStaysSymmetric(t *testing.T) {
	t.Parallel()

	const n = 41

	s := sim.NewWave(n, n)
	s.Poke(n/2, n/2, 1)
	s.Step(sim.BackendGo, 60)

	f := s.Field()

	// Scale the tolerance to the field's peak, not to each cell: a cell whose
	// own value is near zero is all rounding noise, and a relative test there
	// measures nothing.
	var peak float64
	for _, v := range f {
		peak = math.Max(peak, math.Abs(float64(v)))
	}

	tol := 1e-5 * peak

	for y := range n {
		for x := range n {
			v := f[y*n+x]
			for _, m := range [][2]int{{n - 1 - x, y}, {x, n - 1 - y}, {y, x}} {
				got := f[m[1]*n+m[0]]
				if math.Abs(float64(got-v)) > tol {
					t.Fatalf("(%d, %d)=%v not mirrored at (%d, %d)=%v", x, y, v, m[0], m[1], got)
				}
			}
		}
	}
}

// Damped, and well inside the Courant limit, the pond must lose energy rather
// than blow up -- the failure mode of a mistuned explicit scheme.
func TestWaveIsStable(t *testing.T) {
	t.Parallel()

	s := sim.NewWave(64, 64)
	s.Poke(32, 32, 1)
	s.Step(sim.BackendGo, 20)

	energy := func() float64 {
		var e float64
		for _, v := range s.Field() {
			e += float64(v) * float64(v)
		}

		return e
	}

	start := energy()

	s.Step(sim.BackendGo, 4000)

	end := energy()
	if math.IsNaN(end) || math.IsInf(end, 0) {
		t.Fatalf("energy diverged to %v", end)
	}

	if end > start {
		t.Fatalf("energy grew from %v to %v; the scheme should be damped", start, end)
	}
}

// All three backends resolve to the same Go code in a native build, so this
// asserts the dispatch wiring rather than the kernels. The kernels themselves
// are cross-checked inside the wasm demo, which is the only place they exist.
func TestBackendsAgree(t *testing.T) {
	t.Parallel()

	fields := make([][]float32, 0, len(sim.Backends()))

	for _, b := range sim.Backends() {
		s := sim.NewWave(48, 32)
		s.Poke(20, 16, 1)
		s.Step(b, 50)
		fields = append(fields, append([]float32(nil), s.Field()...))
	}

	for i := 1; i < len(fields); i++ {
		for j := range fields[i] {
			if fields[i][j] != fields[0][j] {
				t.Fatalf("%v differs from %v at %d: %v vs %v",
					sim.Backends()[i], sim.Backends()[0], j, fields[i][j], fields[0][j])
			}
		}
	}
}

func TestScalarColormap(t *testing.T) {
	t.Parallel()

	field := []float32{0, 1, -1, 10, -10}
	rgba := make([]byte, len(field)*4)
	simd.ScalarColormap(rgba, field, 1)

	want := [][4]byte{
		{0, 0, 0, 255},      // zero -> black
		{255, 176, 48, 255}, // +1 -> amber
		{56, 148, 255, 255}, // -1 -> blue
		{255, 176, 48, 255}, // clamped high
		{56, 148, 255, 255}, // clamped low
	}

	for i, w := range want {
		got := [4]byte(rgba[i*4 : i*4+4])
		if got != w {
			t.Fatalf("field[%d]=%v: got %v, want %v", i, field[i], got, w)
		}
	}
}
