package simd

import "testing"

// These run natively (non-wasm), so they exercise the pure-Go fallback that
// ships whenever the gosimd build tag is absent. The SIMD kernels themselves
// are verified by the demo program under host/run.mjs, because `go test`
// cannot supply the extra "gosimd" import module to the test binary.

func TestDot(t *testing.T) {
	t.Parallel()

	a := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9}
	if got, want := Dot(a, a), float32(285); got != want {
		t.Errorf("Dot = %v, want %v", got, want)
	}

	if got := Dot(nil, nil); got != 0 {
		t.Errorf("Dot(nil, nil) = %v, want 0", got)
	}

	if got, want := Dot(a, a[:2]), float32(5); got != want {
		t.Errorf("Dot with ragged lengths = %v, want %v", got, want)
	}
}

func TestAdd(t *testing.T) {
	t.Parallel()

	a := []float32{1, 2, 3, 4, 5}
	dst := make([]float32, 5)
	Add(dst, a, a)

	for i, v := range dst {
		if want := a[i] * 2; v != want {
			t.Errorf("Add[%d] = %v, want %v", i, v, want)
		}
	}
}

func TestScalarMatchesDispatch(t *testing.T) {
	t.Parallel()

	a := []float32{0.5, -1.5, 2.25, 3}
	if Dot(a, a) != ScalarDot(a, a) {
		t.Error("Dot and ScalarDot disagree")
	}
}
