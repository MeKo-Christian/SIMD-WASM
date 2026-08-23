//go:build wasm && gosimd

package simd

import (
	"runtime"
	"unsafe"
)

const enabled = true

// Two side modules, both compiled from kernel/kernel.c and both importing Go's
// linear memory, are bound here under different import module names:
//
//	gosimd    build/kernel.wasm         clang -msimd128, the v128 path
//	gocscalar build/kernel-scalar.wasm  clang without simd128, same source
//
// The second one exists so the demo can separate two effects that the headline
// speedup otherwise conflates: how much SIMD wins, and how much clang's wasm
// codegen already beats Go's.

//go:wasmimport gosimd dot_f32
func kernelDotF32(a, b unsafe.Pointer, n int32) float32

//go:wasmimport gosimd add_f32
func kernelAddF32(dst, a, b unsafe.Pointer, n int32)

//go:wasmimport gosimd wave_step_f32
func kernelWaveStepF32(next, cur, prev unsafe.Pointer, w, h int32, c2, damp float32)

//go:wasmimport gosimd colormap_f32
func kernelColormapF32(rgba, field unsafe.Pointer, n int32, scale float32)

//go:wasmimport gocscalar dot_f32
func cKernelDotF32(a, b unsafe.Pointer, n int32) float32

//go:wasmimport gocscalar add_f32
func cKernelAddF32(dst, a, b unsafe.Pointer, n int32)

//go:wasmimport gocscalar wave_step_f32
func cKernelWaveStepF32(next, cur, prev unsafe.Pointer, w, h int32, c2, damp float32)

//go:wasmimport gocscalar colormap_f32
func cKernelColormapF32(rgba, field unsafe.Pointer, n int32, scale float32)

// Dot returns the dot product of a and b, using min(len(a), len(b)) elements.
func Dot(a, b []float32) float32 { return dot(a, b, true) }

// CDot is Dot on the clang-scalar kernel.
func CDot(a, b []float32) float32 { return dot(a, b, false) }

// The kernels are invoked from a direct call site rather than through a func
// value: //go:wasmimport declarations are stubs with a native wasm signature,
// and keeping every call literal is what lets `go vet` apply its wasmimport
// pointer rules to them.
func dot(a, b []float32, vector bool) float32 {
	n := min(len(a), len(b))
	if n == 0 {
		return 0
	}

	ptrA, ptrB := unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0])

	var result float32
	if vector {
		result = kernelDotF32(ptrA, ptrB, int32(n))
	} else {
		result = cKernelDotF32(ptrA, ptrB, int32(n))
	}

	runtime.KeepAlive(a)
	runtime.KeepAlive(b)

	return result
}

// Add computes dst = a + b elementwise over min(len(dst), len(a), len(b)).
func Add(dst, a, b []float32) { addWith(dst, a, b, true) }

// CAdd is Add on the clang-scalar kernel.
func CAdd(dst, a, b []float32) { addWith(dst, a, b, false) }

func addWith(dst, a, b []float32, vector bool) {
	n := min(len(dst), len(a), len(b))
	if n == 0 {
		return
	}

	ptrDst, ptrA, ptrB := unsafe.Pointer(&dst[0]), unsafe.Pointer(&a[0]), unsafe.Pointer(&b[0])
	if vector {
		kernelAddF32(ptrDst, ptrA, ptrB, int32(n))
	} else {
		cKernelAddF32(ptrDst, ptrA, ptrB, int32(n))
	}

	runtime.KeepAlive(dst)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
}

// WaveStep advances one leapfrog step of the 2D wave equation. See
// ScalarWaveStep for the stencil and the grid contract.
func WaveStep(next, cur, prev []float32, w, h int, c2, damp float32) {
	waveStep(next, cur, prev, w, h, c2, damp, true)
}

// CWaveStep is WaveStep on the clang-scalar kernel.
func CWaveStep(next, cur, prev []float32, w, h int, c2, damp float32) {
	waveStep(next, cur, prev, w, h, c2, damp, false)
}

func waveStep(next, cur, prev []float32, w, h int, c2, damp float32, vector bool) {
	n := w * h
	if w < 3 || h < 3 || len(next) < n || len(cur) < n || len(prev) < n {
		return
	}

	ptrNext := unsafe.Pointer(&next[0])
	ptrCur := unsafe.Pointer(&cur[0])
	ptrPrev := unsafe.Pointer(&prev[0])

	if vector {
		kernelWaveStepF32(ptrNext, ptrCur, ptrPrev, int32(w), int32(h), c2, damp)
	} else {
		cKernelWaveStepF32(ptrNext, ptrCur, ptrPrev, int32(w), int32(h), c2, damp)
	}

	runtime.KeepAlive(next)
	runtime.KeepAlive(cur)
	runtime.KeepAlive(prev)
}

// Colormap turns a scalar field into RGBA bytes. See ScalarColormap.
func Colormap(rgba []byte, field []float32, scale float32) {
	colormap(rgba, field, scale, true)
}

// CColormap is Colormap on the clang-scalar kernel.
func CColormap(rgba []byte, field []float32, scale float32) {
	colormap(rgba, field, scale, false)
}

func colormap(rgba []byte, field []float32, scale float32, vector bool) {
	n := min(len(field), len(rgba)/4)
	if n == 0 {
		return
	}

	ptrRGBA, ptrField := unsafe.Pointer(&rgba[0]), unsafe.Pointer(&field[0])
	if vector {
		kernelColormapF32(ptrRGBA, ptrField, int32(n), scale)
	} else {
		cKernelColormapF32(ptrRGBA, ptrField, int32(n), scale)
	}

	runtime.KeepAlive(rgba)
	runtime.KeepAlive(field)
}
