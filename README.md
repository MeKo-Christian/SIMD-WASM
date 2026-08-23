# SIMD in WebAssembly from standard Go

A proof of concept: real WebAssembly SIMD (`v128`) instructions driven from Go
code built by the **stock Go toolchain** — no TinyGo, no compiler fork, no
binary rewriting.

## The problem

The standard Go compiler cannot produce wasm SIMD, by any route:

| Route                     | Status (Go 1.26)                                               |
| ------------------------- | -------------------------------------------------------------- |
| `simd` intrinsics package | `src/simd/archsimd` is `//go:build goexperiment.simd && amd64` |
| Auto-vectorization        | The wasm backend does not vectorize                            |
| Hand-written assembly     | `cmd/internal/obj/wasm` has no `0xFD` / `v128` opcodes at all  |

So the trick is to stop asking the Go compiler for SIMD.

## How it works

The kernels are compiled separately by clang (`-msimd128`) into a small side
module that **imports Go's linear memory** instead of defining its own. Go
declares the kernels with `//go:wasmimport` — the one place the toolchain hands
you a native wasm signature (`i32`/`f32` params) instead of Go's
stack-in-memory calling convention — and passes raw slice pointers.

Because both modules share one `WebAssembly.Memory`, the kernel reads and
writes Go slices **in place**: no copying, no serialization.

```
  Go module (standard toolchain)          kernel.wasm (clang -msimd128)
  ┌───────────────────────────┐           ┌──────────────────────────┐
  │ //go:wasmimport gosimd    │  import   │ export dot_f32, add_f32, │
  │ func kernelDotF32(...)  ──┼──────────▶│   wave_step_f32, …       │
  │                           │           │   f32x4.mul / f32x4.add  │
  │ //go:wasmimport gocscalar │  import   │                          │
  │ func cKernelDotF32(...) ──┼───────┐   │ imports env.memory       │
  │                           │       │   └──────────────────────────┘
  │ exports memory ───────────┼───────┼──────────────▲
  └───────────────────────────┘       │              │
                                      ▼              │
                        kernel-scalar.wasm (same C, no simd128)
            all instances address the same bytes
```

The modules import each other, so the host wires them in order: instantiate
Go (it defines the memory), instantiate the kernels against that memory, then
start Go. Go's imports point at trampolines that are resolved in between —
see `host/run.mjs` and `host/kernels.mjs`.

## Three backends, one binary

`kernel/kernel.c` is compiled **twice**: once with `-msimd128` and once
without. Both modules export the same names and are bound under different
import module names (`gosimd` and `gocscalar`), so one Go binary can run three
implementations of every kernel over identical data:

| backend      | what it is                             |
| ------------ | -------------------------------------- |
| **Go**       | the pure-Go loop, stock wasm backend   |
| **C**        | the same C source, clang, no `simd128` |
| **C + SIMD** | the same C source, clang `-msimd128`   |

That middle column is the honest part. A "SIMD is 10× faster" headline mixes
two effects, and only one of them is SIMD: **_vs C_ is what vectorising won,
_vs Go_ also counts clang out-compiling Go's wasm backend.**

## Results

Measured on `wasip1` under Node, n=4096 float32 for the vector kernels and a
512×512 grid for the stencil. `vs Go` and `vs C` are both against `C + SIMD`:

| kernel      | Go       | C       | C + SIMD | vs Go     | vs C      |
| ----------- | -------- | ------- | -------- | --------- | --------- |
| `Dot`       | 4.91 µs  | 2.23 µs | 0.71 µs  | **6.96×** | **3.16×** |
| `Add`       | 4.85 µs  | 1.82 µs | 0.61 µs  | **7.96×** | **2.98×** |
| `Wave 512²` | 1.134 ms | 453 µs  | 179 µs   | **6.34×** | **2.53×** |

Read the `vs C` column as the SIMD result. It lands under the 4× that 128-bit
lanes allow on f32 — the elementwise kernels are limited by memory bandwidth
rather than arithmetic, and the stencil more so, since each cell is read five
times. The remaining factor of ~2.5 in `vs Go` is Go's wasm codegen: bounds
checks and no unrolling.

Verified: identical results vs. the pure-Go reference; the stencil cross-checked
across all three backends after 200 compounding steps; correct after forcing
~64 MiB of Go heap growth _after_ the kernels were bound (growth is shared,
because it is the same `Memory` object); clean `go vet` including the
`wasmimport` pointer checks; and running in real Chrome, not just Node.

## Build and run

Requires Go 1.26, clang with the wasm32 target, `wasm-ld`, Node and
[`just`](https://github.com/casey/just).

Use Node 24.x (or 22.21.0). **Node 22.21.1 through at least 22.23.2 segfault**
partway through the wasip1 demo: a V8 concurrent-marking bug fires once Go's
linear memory grows under `node:wasi`. It is not specific to this code — the
pure-Go build crashes the same way with no kernel loaded — and it does not
affect the `GOOS=js` path or the browser. `host/run.mjs` warns when it sees an
affected version.

```sh
just build       # both kernel modules + all three app builds
just run         # wasip1, SIMD
just run-plain   # wasip1, pure-Go fallback
just run-js      # GOOS=js under Node
just serve       # browser demo on :8080
just site        # same demo as a static directory, ready for any host
just test        # native tests of the fallback path
just verify      # assert the built modules are what they claim to be
just             # every recipe, with descriptions
```

`just check` runs the static checks: formatting, `golangci-lint` natively and
under the wasm build tags, `go vet` both ways, and the tests. Formatting is
[treefmt](https://github.com/numtide/treefmt)-driven (`just fmt`); `just
setup-deps` installs the formatters and linters it needs.

## Layout

```
kernel/kernel.c     the kernels; vector paths behind #ifdef __wasm_simd128__
justfile            build recipes: clang --target=wasm32 [-msimd128], go build
simd/               Go API: wasmimport kernels + pure-Go fallback
sim/                the wave simulation, natively testable
host/kernels.mjs    the kernel export list and import trampolines
host/run.mjs        wasip1 host wiring (Node WASI)
host/run-js.mjs     GOOS=js host wiring (wasm_exec.js)
host/index.html     browser demo
cmd/serve           static server for the browser demo
```

## Constraints, honestly

- **The host must cooperate.** The Go module alone is not self-contained; the
  embedder supplies the `gosimd` import. Fine when you own the page or the
  wasm runtime. To ship one standalone `.wasm`, merge the modules at build
  time with binaryen's `wasm-merge` (not installed here) — the wiring is
  identical, resolved statically instead of at instantiation.
- **One C source, two builds.** The vector paths sit behind
  `#ifdef __wasm_simd128__` and share the scalar tail loop, so the `C` and
  `C + SIMD` columns really are the same algorithm and differ only in the
  instruction set clang was allowed to use.
- **Build with `-tags gosimd`.** Without it you get the pure-Go fallback and an
  ordinary module with zero extra imports, so the same source works anywhere.
- **Pointer discipline.** Pass pointers into heap-allocated slices and
  `runtime.KeepAlive` them across the call. Go's GC does not move heap objects,
  which is what makes this sound.
- **The kernel does no bounds checking.** Lengths are computed on the Go side.
- **Trampoline cost.** Each call crosses into JS and back — tens of
  nanoseconds. Amortized fine at n=4096; pointless for tiny vectors.
- **The kernel must be freestanding** (`-nostdlib -ffreestanding --no-entry`)
  and must not touch a C stack, since it shares Go's address space.
- **128-bit only.** wasm SIMD has no AVX-class width, and no runtime feature
  detection here — a host without simd128 fails at kernel instantiation, so
  fall back by simply not wiring the import.

## The demo on the web

The page runs a **2D wave equation** — a pond you can drop stones into. Each
step applies the 5-point discrete Laplacian and a leapfrog update over a
float32 grid, which is the archetypal SIMD workload: unit-stride, arithmetic
per cell, no branches. The stencil is the float32 counterpart of
[`algo-pde`](https://github.com/cwbudde/algo-pde)'s `fd.Apply2D`; the explicit
time stepper around it is new here, since `algo-pde` itself is an elliptic
spectral library and does no time stepping.

Three ponds run side by side, one per backend, from identical initial state:

- **equal time** — every panel gets the same milliseconds per frame and takes
  as many steps as it can. Nobody has to read a number: the fast ponds are
  simply further along, and churn visibly faster.
- **equal work** — every panel takes the same number of steps, so all three
  render the same picture (a correctness check you can see) and the difference
  shows up as `ms/step`.

The **Microbench** tab is the table above, run in the browser on demand.

`.github/workflows/build-deploy-web.yaml` publishes it to GitHub Pages on every
push to `main`: it runs `just site`, which drops `host/index.html`,
`host/kernels.mjs`, `build/app-simd-js.wasm`, both kernel modules and Go's
`wasm_exec.js` into one flat directory, and uploads that as the Pages artifact.
`cmd/serve` has no part in it — Pages serves static files only, and everything
the page needs is a plain fetch of a file next to it.

It only works once Pages is enabled for the repository with **Source: GitHub
Actions** (Settings → Pages); until then the deploy job fails with a 404.

## Continuous integration

`.github/workflows/test.yaml` fans out to one reusable workflow per concern —
`test-format`, `test-lint`, `test-unit`, `test-build`, `test-demo` — so a
failure names itself in the checks list instead of hiding inside one long job.
Each of them installs its toolchain through the composite action in
`.github/actions/setup` and then calls the same `just` recipes you run locally
(`just check-formatted`, `just lint` / `lint-wasm`, `just vet` / `vet-wasm`,
`just test`, `just build`, `just verify` and `just run-js`). Locally, `just ci`
runs all of it in one shot.

The `go vet` and `golangci-lint` runs happen twice, natively and under the wasm
build tags, because the `//go:wasmimport` declarations — and the pointer rules
that apply to them — only exist in the tagged build.

The demos exit non-zero when a check fails, so a wrong kernel breaks the build
rather than printing `FAIL` into a green log. `just verify` additionally asserts
that the kernel really carries the `simd128` target feature, that the tagged
build reports its kernels as linked, and that the untagged build contains no
`gosimd` import at all.

## License

MIT — see [LICENSE](LICENSE).
