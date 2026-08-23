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
package simd

// Enabled reports whether the SIMD kernels are linked in.
func Enabled() bool { return enabled }
