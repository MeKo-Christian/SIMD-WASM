// Package simd exposes vector kernels that run as real WebAssembly SIMD
// (v128) instructions, from code compiled by the standard Go toolchain.
//
// The standard Go compiler cannot emit SIMD for GOARCH=wasm: there are no
// intrinsics, the wasm backend does not auto-vectorize, and the wasm
// assembler has no 0xFD opcodes. This package sidesteps that by not asking
// the Go compiler for SIMD at all. The kernels are compiled separately by
// clang (-msimd128) into a tiny side module that imports Go's linear memory;
// Go declares them with //go:wasmimport and passes raw pointers.
//
// Build with -tags gosimd to use the kernels; without the tag you get the
// pure-Go implementations and an ordinary, self-contained module.
//
// Two side modules are bound, not one: kernel.wasm (clang -msimd128) and
// kernel-scalar.wasm (the same C source, no simd128). Having the clang-scalar
// path in the same binary is what lets the demo separate the SIMD win from
// clang's codegen advantage over Go's wasm backend.
package simd

// Enabled reports whether the SIMD kernels are linked in.
func Enabled() bool { return enabled }

// CEnabled reports whether the clang-scalar kernels are linked in. It tracks
// Enabled today -- both modules are wired by the same build tag -- but the host
// supplies them independently, so the demo asks about them separately.
func CEnabled() bool { return enabled }
