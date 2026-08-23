// Host glue: instantiate the Go module, then instantiate the SIMD kernel
// module on top of Go's *own* linear memory, and satisfy Go's "gosimd"
// imports from the kernel's exports.
//
// The two modules reference each other, so the wiring is: create the Go
// instance first (it defines and exports the memory), give it trampoline
// stubs for the kernels, then instantiate the kernel module against that
// same memory and point the trampolines at it -- all before _start runs.
import { readFile } from 'node:fs/promises';
import { WASI } from 'node:wasi';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const appPath = process.argv[2] ?? join(root, 'build', 'app-simd.wasm');
const withKernel = !process.argv.includes('--no-kernel');

const wasi = new WASI({ version: 'preview1', args: ['app'], env: process.env });

let kernel = null; // filled in after the Go instance exists
const imports = { ...wasi.getImportObject() };
if (withKernel) {
  imports.gosimd = {
    dot_f32: (a, b, n) => kernel.dot_f32(a, b, n),
    add_f32: (dst, a, b, n) => kernel.add_f32(dst, a, b, n),
  };
}

const app = await WebAssembly.instantiate(
  await WebAssembly.compile(await readFile(appPath)), imports);

if (withKernel) {
  const mod = await WebAssembly.compile(await readFile(join(root, 'build', 'kernel.wasm')));
  const inst = await WebAssembly.instantiate(mod, { env: { memory: app.exports.memory } });
  kernel = inst.exports;
}

const code = wasi.start(app);
if (code) process.exit(code);
