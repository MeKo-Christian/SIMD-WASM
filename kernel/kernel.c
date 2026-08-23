// SIMD kernels compiled by clang to WebAssembly with the simd128 feature.
// Linked with --import-memory so they operate directly on Go's linear memory:
// pointers handed over from Go are plain i32 offsets into that same memory.
#include <wasm_simd128.h>

__attribute__((export_name("dot_f32")))
float dot_f32(const float *a, const float *b, int n) {
	v128_t acc = wasm_f32x4_const_splat(0.0f);
	int i = 0;
	for (; i + 4 <= n; i += 4) {
		// unaligned loads: Go slices are 4-byte aligned, not 16
		v128_t x = wasm_v128_load(a + i);
		v128_t y = wasm_v128_load(b + i);
		acc = wasm_f32x4_add(acc, wasm_f32x4_mul(x, y));
	}
	float s = wasm_f32x4_extract_lane(acc, 0) + wasm_f32x4_extract_lane(acc, 1) +
	          wasm_f32x4_extract_lane(acc, 2) + wasm_f32x4_extract_lane(acc, 3);
	for (; i < n; i++) s += a[i] * b[i];
	return s;
}

__attribute__((export_name("add_f32")))
void add_f32(float *dst, const float *a, const float *b, int n) {
	int i = 0;
	for (; i + 4 <= n; i += 4)
		wasm_v128_store(dst + i, wasm_f32x4_add(wasm_v128_load(a + i), wasm_v128_load(b + i)));
	for (; i < n; i++) dst[i] = a[i] + b[i];
}
