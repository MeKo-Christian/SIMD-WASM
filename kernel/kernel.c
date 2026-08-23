// Compute kernels compiled by clang to WebAssembly, twice from this one file:
// once with -msimd128 (build/kernel.wasm) and once without
// (build/kernel-scalar.wasm). Both are linked with --import-memory so they
// operate directly on Go's linear memory: pointers handed over from Go are
// plain i32 offsets into that same memory.
//
// The vector paths live behind #ifdef __wasm_simd128__ and always share the
// scalar tail loop, which is also the whole loop in the no-simd128 build. That
// keeps one source of truth per algorithm, so the demo's "C" and "C + SIMD"
// columns differ only in the instruction set clang was allowed to use.
#ifdef __wasm_simd128__
#include <wasm_simd128.h>
#endif

#define EXPORT(name) __attribute__((export_name(name)))

EXPORT("dot_f32")
float dot_f32(const float *a, const float *b, int n) {
	int i = 0;
	float s = 0.0f;
#ifdef __wasm_simd128__
	v128_t acc = wasm_f32x4_const_splat(0.0f);
	for (; i + 4 <= n; i += 4) {
		// unaligned loads: Go slices are 4-byte aligned, not 16
		v128_t x = wasm_v128_load(a + i);
		v128_t y = wasm_v128_load(b + i);
		acc = wasm_f32x4_add(acc, wasm_f32x4_mul(x, y));
	}
	s = wasm_f32x4_extract_lane(acc, 0) + wasm_f32x4_extract_lane(acc, 1) +
	    wasm_f32x4_extract_lane(acc, 2) + wasm_f32x4_extract_lane(acc, 3);
#endif
	for (; i < n; i++) s += a[i] * b[i];
	return s;
}

EXPORT("add_f32")
void add_f32(float *dst, const float *a, const float *b, int n) {
	int i = 0;
#ifdef __wasm_simd128__
	for (; i + 4 <= n; i += 4)
		wasm_v128_store(dst + i, wasm_f32x4_add(wasm_v128_load(a + i), wasm_v128_load(b + i)));
#endif
	for (; i < n; i++) dst[i] = a[i] + b[i];
}

// wave_step_f32 advances one leapfrog step of the 2D wave equation on a w*h
// grid stored row-major:
//
//	next = damp * (2*cur - prev + c2 * (N + S + E + W - 4*cur))
//
// The bracketed term is the 5-point discrete Laplacian. The one-cell border is
// held at zero (Dirichlet), so only the interior is written; callers zero next
// once at allocation time and it stays zero there.
EXPORT("wave_step_f32")
void wave_step_f32(float *next, const float *cur, const float *prev, int w, int h, float c2,
                   float damp) {
	if (w < 3 || h < 3) return;

	for (int y = 1; y < h - 1; y++) {
		const float *c = cur + (long)y * w;
		const float *up = c - w;
		const float *dn = c + w;
		const float *p = prev + (long)y * w;
		float *o = next + (long)y * w;
		int x = 1;
#ifdef __wasm_simd128__
		const v128_t vc2 = wasm_f32x4_splat(c2);
		const v128_t vdamp = wasm_f32x4_splat(damp);
		const v128_t two = wasm_f32x4_const_splat(2.0f);
		const v128_t four = wasm_f32x4_const_splat(4.0f);
		for (; x + 4 <= w - 1; x += 4) {
			v128_t mid = wasm_v128_load(c + x);
			v128_t lap = wasm_f32x4_add(wasm_v128_load(c + x - 1), wasm_v128_load(c + x + 1));
			lap = wasm_f32x4_add(lap, wasm_f32x4_add(wasm_v128_load(up + x), wasm_v128_load(dn + x)));
			lap = wasm_f32x4_sub(lap, wasm_f32x4_mul(four, mid));

			v128_t v = wasm_f32x4_sub(wasm_f32x4_mul(two, mid), wasm_v128_load(p + x));
			v = wasm_f32x4_add(v, wasm_f32x4_mul(vc2, lap));
			wasm_v128_store(o + x, wasm_f32x4_mul(vdamp, v));
		}
#endif
		for (; x < w - 1; x++) {
			float mid = c[x];
			float lap = c[x - 1] + c[x + 1] + up[x] + dn[x] - 4.0f * mid;
			o[x] = damp * (2.0f * mid - p[x] + c2 * lap);
		}
	}
}

// colormap_f32 turns a scalar field into RGBA bytes with a diverging blue /
// black / amber ramp: negative values go blue, positive amber, zero black.
// t = clamp(field * scale, -1, 1) selects the ramp position.
//
// This is presentation, not the benchmarked work -- every panel renders through
// the same call so the comparison stays about the stepper.
EXPORT("colormap_f32")
void colormap_f32(unsigned char *rgba, const float *field, int n, float scale) {
	int i = 0;
#ifdef __wasm_simd128__
	const v128_t vscale = wasm_f32x4_splat(scale);
	const v128_t one = wasm_f32x4_const_splat(1.0f);
	const v128_t neg1 = wasm_f32x4_const_splat(-1.0f);
	const v128_t zero = wasm_f32x4_const_splat(0.0f);
	const v128_t alpha = wasm_i32x4_const_splat(0xFF000000);
	for (; i + 4 <= n; i += 4) {
		v128_t t = wasm_v128_load(field + i);
		t = wasm_f32x4_pmin(one, wasm_f32x4_pmax(neg1, wasm_f32x4_mul(t, vscale)));
		v128_t pos = wasm_f32x4_pmax(zero, t);
		v128_t neg = wasm_f32x4_pmax(zero, wasm_f32x4_sub(zero, t));

		v128_t r = wasm_f32x4_add(wasm_f32x4_mul(pos, wasm_f32x4_const_splat(255.0f)),
		                          wasm_f32x4_mul(neg, wasm_f32x4_const_splat(56.0f)));
		v128_t g = wasm_f32x4_add(wasm_f32x4_mul(pos, wasm_f32x4_const_splat(176.0f)),
		                          wasm_f32x4_mul(neg, wasm_f32x4_const_splat(148.0f)));
		v128_t b = wasm_f32x4_add(wasm_f32x4_mul(pos, wasm_f32x4_const_splat(48.0f)),
		                          wasm_f32x4_mul(neg, wasm_f32x4_const_splat(255.0f)));

		// RGBA is little-endian in memory, so pixel = R | G<<8 | B<<16 | A<<24
		// packs four whole pixels into one v128 and stores them in one go.
		v128_t px = wasm_v128_or(wasm_i32x4_trunc_sat_f32x4(r), alpha);
		px = wasm_v128_or(px, wasm_i32x4_shl(wasm_i32x4_trunc_sat_f32x4(g), 8));
		px = wasm_v128_or(px, wasm_i32x4_shl(wasm_i32x4_trunc_sat_f32x4(b), 16));
		wasm_v128_store(rgba + (long)i * 4, px);
	}
#endif
	for (; i < n; i++) {
		float t = field[i] * scale;
		if (t > 1.0f) t = 1.0f;
		if (t < -1.0f) t = -1.0f;
		float pos = t > 0.0f ? t : 0.0f;
		float neg = t < 0.0f ? -t : 0.0f;
		unsigned char *px = rgba + (long)i * 4;
		px[0] = (unsigned char)(pos * 255.0f + neg * 56.0f);
		px[1] = (unsigned char)(pos * 176.0f + neg * 148.0f);
		px[2] = (unsigned char)(pos * 48.0f + neg * 255.0f);
		px[3] = 255;
	}
}
