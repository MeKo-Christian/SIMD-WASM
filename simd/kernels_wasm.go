//go:build wasm && gosimd

package simd

import (
	"runtime"
	"unsafe"
)

const enabled = true

//go:wasmimport gosimd dot_f32
func kernelDotF32(a, b unsafe.Pointer, n int32) float32

//go:wasmimport gosimd add_f32
func kernelAddF32(dst, a, b unsafe.Pointer, n int32)

// Dot returns the dot product of a and b, using min(len(a), len(b)) elements.
func Dot(a, b []float32) float32 {
	n := min(len(a), len(b))
	if n == 0 {
		return 0
	}
	r := kernelDotF32(unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int32(n))
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)

	return r
}

// Add computes dst = a + b elementwise over min(len(dst), len(a), len(b)).
func Add(dst, a, b []float32) {
	n := min(len(dst), len(a), len(b))
	if n == 0 {
		return
	}

	kernelAddF32(unsafe.Pointer(&dst[0]), unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0]), int32(n))
	runtime.KeepAlive(dst)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
}
