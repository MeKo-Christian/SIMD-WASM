package simd

// ScalarDot and ScalarAdd are the pure-Go reference implementations. They are
// always available so the demo can compare them against the SIMD kernels in
// the same binary.

func ScalarDot(a, b []float32) float32 {
	n := min(len(a), len(b))
	a, b = a[:n], b[:n]
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

func ScalarAdd(dst, a, b []float32) {
	n := min(len(dst), len(a), len(b))
	dst, a, b = dst[:n], a[:n], b[:n]
	for i := range dst {
		dst[i] = a[i] + b[i]
	}
}
