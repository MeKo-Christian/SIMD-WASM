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

The kernels are compiled separately by clang (`-msimd128`) into a ~1 KB side
module that **imports Go's linear memory** instead of defining its own. Go
declares the kernels with `//go:wasmimport` — the one place the toolchain hands
you a native wasm signature (`i32`/`f32` params) instead of Go's
stack-in-memory calling convention — and passes raw slice pointers.

Because both modules share one `WebAssembly.Memory`, the kernel reads and
writes Go slices **in place**: no copying, no serialization.

```
  Go module (standard toolchain)          kernel.wasm (clang -msimd128)
  ┌───────────────────────────┐           ┌──────────────────────────┐
  │ //go:wasmimport gosimd    │  import   │ export dot_f32, add_f32  │
  │ func kernelDotF32(...)  ──┼──────────▶│   f32x4.mul / f32x4.add  │
  │                           │           │                          │
  │ exports memory ───────────┼──────────▶│ imports env.memory       │
  └───────────────────────────┘           └──────────────────────────┘
            both instances address the same bytes
```

The two modules import each other, so the host wires them in order: instantiate
Go (it defines the memory), instantiate the kernel against that memory, then
start Go. Go's imports point at trampolines that are resolved in between —
see `host/run.mjs`.

## Results

Measured here on `wasip1` under Node, n=4096 float32, 2000 iterations:

| kernel | scalar Go | SIMD    | speedup    |
| ------ | --------- | ------- | ---------- |
| `Dot`  | 5.04 µs   | 1.59 µs | **9.98×**  |
| `Add`  | 4.63 µs   | 0.82 µs | **11.37×** |

Above 4× because wasm SIMD is 128-bit (4× on f32) _and_ Go's scalar wasm
codegen is weak — bounds checks and no unrolling. Don't expect this ratio
against well-optimized scalar code on other targets.

Verified: identical results vs. the pure-Go reference; correct after forcing
~64 MiB of Go heap growth _after_ the kernel was bound (growth is shared,
because it is the same `Memory` object); clean `go vet` including the
`wasmimport` pointer checks; and running in real Chrome, not just Node.

## Build and run

Requires Go 1.26, clang with the wasm32 target, `wasm-ld`, Node ≥ 20 and
[`just`](https://github.com/casey/just).

```sh
just build       # kernel + all three app builds
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
kernel/kernel.c     SIMD kernels (wasm_simd128.h intrinsics)
justfile            build recipes: clang --target=wasm32 -msimd128, go build
simd/               Go API: wasmimport kernels + pure-Go fallback
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

`.github/workflows/build-deploy-web.yaml` publishes the browser demo to GitHub
Pages on every push to `main`: it runs `just site`, which drops `host/index.html`,
`build/app-simd-js.wasm`, `build/kernel.wasm` and Go's `wasm_exec.js` into one
flat directory, and uploads that as the Pages artifact. `cmd/serve` has no part
in it — Pages serves static files only, and everything the page needs is a plain
fetch of a file next to it.

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
