//go:build !(wasm && gosimd)

package simd

// Without the gosimd tag -- or off wasm entirely -- there are no side modules
// to call, so every backend resolves to the pure-Go reference. The same source
// therefore builds and runs anywhere, and `go test` can exercise the logic
// natively.

const enabled = false

// Dot returns the dot product of a and b, using min(len(a), len(b)) elements.
func Dot(a, b []float32) float32 { return ScalarDot(a, b) }

// CDot is Dot on the clang-scalar kernel.
func CDot(a, b []float32) float32 { return ScalarDot(a, b) }

// Add computes dst = a + b elementwise over min(len(dst), len(a), len(b)).
func Add(dst, a, b []float32) { ScalarAdd(dst, a, b) }

// CAdd is Add on the clang-scalar kernel.
func CAdd(dst, a, b []float32) { ScalarAdd(dst, a, b) }

// WaveStep advances one leapfrog step of the 2D wave equation. See
// ScalarWaveStep for the stencil and the grid contract.
func WaveStep(next, cur, prev []float32, w, h int, c2, damp float32) {
	ScalarWaveStep(next, cur, prev, w, h, c2, damp)
}

// CWaveStep is WaveStep on the clang-scalar kernel.
func CWaveStep(next, cur, prev []float32, w, h int, c2, damp float32) {
	ScalarWaveStep(next, cur, prev, w, h, c2, damp)
}

// Colormap turns a scalar field into RGBA bytes. See ScalarColormap.
func Colormap(rgba []byte, field []float32, scale float32) {
	ScalarColormap(rgba, field, scale)
}

// CColormap is Colormap on the clang-scalar kernel.
func CColormap(rgba []byte, field []float32, scale float32) {
	ScalarColormap(rgba, field, scale)
}
