// One list of kernel exports, shared by every host: the two Node runners and
// the browser page. Adding a kernel means adding its name here and nowhere
// else on the JS side.
export const KERNEL_EXPORTS = [
  "dot_f32",
  "add_f32",
  "wave_step_f32",
  "colormap_f32",
];

// The Go module and the kernel modules import each other, so Go's imports have
// to be resolvable before any kernel instance exists. Each import is therefore
// a trampoline through a mutable holder, which the host fills in between
// instantiating Go and starting it.
//
// Returns { holder, imports }: pass imports to the Go instantiation, then set
// holder.exports to the kernel instance's exports.
export function kernelImports() {
  const holder = { exports: null };
  const imports = Object.fromEntries(
    KERNEL_EXPORTS.map((name) => [
      name,
      (...args) => holder.exports[name](...args),
    ]),
  );

  return { holder, imports };
}

// The two side modules Go binds: build/kernel.wasm under the import module
// name "gosimd", build/kernel-scalar.wasm under "gocscalar". Both are compiled
// from kernel/kernel.c and both import Go's linear memory; they differ only in
// whether clang was allowed to emit simd128.
export const KERNEL_MODULES = [
  { importName: "gosimd", file: "kernel.wasm" },
  { importName: "gocscalar", file: "kernel-scalar.wasm" },
];
