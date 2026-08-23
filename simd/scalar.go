package simd

// The pure-Go reference implementations. They are always available, in every
// build, so the demo can compare all three backends -- Go, clang scalar and
// clang SIMD -- inside a single binary.

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

// ScalarWaveStep advances one leapfrog step of the 2D wave equation on a w*h
// row-major grid:
//
//	next = damp * (2*cur - prev + c2*(N + S + E + W - 4*cur))
//
// The bracketed term is the 5-point discrete Laplacian, the same stencil as
// algo-pde's fd.Apply2D in float32. The one-cell border is held at zero
// (Dirichlet) and never written, so callers allocate next zeroed and it stays
// zero there.
func ScalarWaveStep(next, cur, prev []float32, w, h int, c2, damp float32) {
	if w < 3 || h < 3 {
		return
	}

	for y := 1; y < h-1; y++ {
		row := y * w
		c := cur[row : row+w : row+w]
		up := cur[row-w : row : row]
		dn := cur[row+w : row+2*w : row+2*w]
		p := prev[row : row+w : row+w]
		o := next[row : row+w : row+w]

		for x := 1; x < w-1; x++ {
			mid := c[x]
			lap := c[x-1] + c[x+1] + up[x] + dn[x] - 4*mid
			o[x] = damp * (2*mid - p[x] + c2*lap)
		}
	}
}

// ScalarColormap turns a scalar field into RGBA bytes with a diverging blue /
// black / amber ramp: negative goes blue, positive amber, zero black.
func ScalarColormap(rgba []byte, field []float32, scale float32) {
	n := min(len(field), len(rgba)/4)
	for i := range n {
		t := field[i] * scale
		t = max(-1, min(1, t))

		pos, neg := max(0, t), max(0, -t)
		px := rgba[i*4 : i*4+4 : i*4+4]
		px[0] = byte(pos*255 + neg*56)
		px[1] = byte(pos*176 + neg*148)
		px[2] = byte(pos*48 + neg*255)
		px[3] = 255
	}
}
