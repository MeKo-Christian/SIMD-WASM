// Same trick, GOOS=js flavour: load Go's wasm_exec.js glue, add the kernel
// import modules to go.importObject, then bind them to Go's exported memory
// (called "mem" on js/wasm, not "memory") before go.run() starts the program.
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { execFileSync } from "node:child_process";
import { KERNEL_MODULES, kernelImports } from "./kernels.mjs";

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
const holders = KERNEL_MODULES.map(({ importName, file }) => {
  const { holder, imports } = kernelImports();
  go.importObject[importName] = imports;

  return { file, holder };
});

const app = await WebAssembly.instantiate(
  await WebAssembly.compile(
    await readFile(join(root, "build", "app-simd-js.wasm")),
  ),
  go.importObject,
);

const instance = app.instance ?? app;

for (const k of holders) {
  k.holder.exports = (
    await WebAssembly.instantiate(
      await WebAssembly.compile(await readFile(join(root, "build", k.file))),
      { env: { memory: instance.exports.mem } },
    )
  ).exports;
}

await go.run(instance);
