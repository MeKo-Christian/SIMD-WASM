//go:build !(wasm && gosimd)

package simd

const enabled = false

// Dot returns the dot product of a and b, using min(len(a), len(b)) elements.
func Dot(a, b []float32) float32 {
	n := min(len(a), len(b))
	a, b = a[:n], b[:n]

	var s float32
	for i := range a {
		s += a[i] * b[i]
	}

	return s
}

// Add computes dst = a + b elementwise over min(len(dst), len(a), len(b)).
func Add(dst, a, b []float32) {
	n := min(len(dst), len(a), len(b))

	dst, a, b = dst[:n], a[:n], b[:n]
	for i := range dst {
		dst[i] = a[i] + b[i]
	}
}
