// Same trick, GOOS=js flavour: load Go's wasm_exec.js glue, add the "gosimd"
// module to go.importObject, then bind the kernel to Go's exported memory
// (called "mem" on js/wasm) before go.run() starts the program.
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { execFileSync } from "node:child_process";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const goroot = execFileSync("go", ["env", "GOROOT"], {
  encoding: "utf8",
}).trim();
createRequire(import.meta.url)(join(goroot, "lib", "wasm", "wasm_exec.js"));

const go = new Go();
// wasm_exec.js only warns on a non-zero exit; make it fail the process.
go.exit = (code) => {
  if (code !== 0) process.exit(code);
};
let kernel = null;
go.importObject.gosimd = {
  dot_f32: (a, b, n) => kernel.dot_f32(a, b, n),
  add_f32: (dst, a, b, n) => kernel.add_f32(dst, a, b, n),
};

const app = await WebAssembly.instantiate(
  await WebAssembly.compile(
    await readFile(join(root, "build", "app-simd-js.wasm")),
  ),
  go.importObject,
);

kernel = (
  await WebAssembly.instantiate(
    await WebAssembly.compile(
      await readFile(join(root, "build", "kernel.wasm")),
    ),
    {
      env: {
        memory: app.instance ? app.instance.exports.mem : app.exports.mem,
      },
    },
  )
).exports;

await go.run(app.instance ?? app);
