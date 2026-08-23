//go:build js && wasm

package main

import (
	"syscall/js"
	"time"

	"github.com/MeKo-Christian/simd-wasm/sim"
	"github.com/MeKo-Christian/simd-wasm/simd"
)

const interactiveSupported = true

// Stepping modes, mirrored by the radio buttons in host/index.html.
const (
	modeEqualTime = "equal-time" // same wall-clock budget, count the steps
	modeEqualWork = "equal-work" // same step count, measure the time
)

// Driver settings for the browser demo. The frequencies are in cycles per
// step, low enough that the emitted wavelength spans tens of cells -- a faster
// drive just makes grid-scale noise. Each source emits for driveBurst steps
// out of every drivePeriod, so the ponds fill with expanding rings whose radii
// show at a glance how far each backend has got.
const (
	driveFreqA  = 0.02
	driveFreqB  = 0.025
	driveAmpl   = 0.6
	driveBurst  = 50
	drivePeriod = 900
)

// panels holds one independent simulation per backend. They are stepped from
// identical state, so in equal-work mode all three render the same picture --
// a correctness check you can see -- and in equal-time mode the faster ones
// simply get further.
var panels []*sim.Wave

// serveInteractive publishes the demo API on globalThis and then blocks: on
// js/wasm, returning from main would tear the instance down and take the
// callbacks with it.
func serveInteractive() {
	global := js.Global()

	global.Set("simdwasmInfo", js.FuncOf(infoFn))
	global.Set("simdwasmInit", js.FuncOf(initFn))
	global.Set("simdwasmReset", js.FuncOf(resetFn))
	global.Set("simdwasmPoke", js.FuncOf(pokeFn))
	global.Set("simdwasmStep", js.FuncOf(stepFn))
	global.Set("simdwasmRender", js.FuncOf(renderFn))
	global.Set("simdwasmBench", js.FuncOf(benchFn))

	// The page waits for this rather than racing go.run().
	if ready := global.Get("simdwasmReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}

	select {}
}

// benchFn runs the benchmark table on demand. Its output goes to stdout, which
// the page is already capturing, so the Microbench tab fills in as it runs --
// and it is not on the page-load path, where several seconds of timing loops
// would just look like a hang.
func benchFn(js.Value, []js.Value) any {
	benchmarks()

	return nil
}

func infoFn(js.Value, []js.Value) any {
	names := make([]any, len(sim.Backends()))
	for i, b := range sim.Backends() {
		names[i] = b.String()
	}

	return map[string]any{
		"simd":     simd.Enabled(),
		"c":        simd.CEnabled(),
		"backends": names,
	}
}

// initFn allocates one w*h pond per backend and drops the opening stone in the
// middle of each, so every panel starts from byte-identical state.
func initFn(_ js.Value, args []js.Value) any {
	w, h := args[0].Int(), args[1].Int()

	panels = make([]*sim.Wave, len(sim.Backends()))
	for i := range panels {
		panels[i] = sim.NewWave(w, h)
		// Two off-centre sources, pulsed, at slightly different frequencies.
		panels[i].Drive(w/3, h/2, driveFreqA, driveAmpl, driveBurst, drivePeriod)
		panels[i].Drive(2*w/3, 2*h/3, driveFreqB, driveAmpl, driveBurst, drivePeriod)
	}

	return nil
}

func resetFn(js.Value, []js.Value) any {
	for _, p := range panels {
		p.Reset()
	}

	return nil
}

// pokeFn drops a stone into every panel at once: the panels must stay
// comparable, so an interaction applies to all of them or to none.
func pokeFn(_ js.Value, args []js.Value) any {
	pokeAll(args[0].Int(), args[1].Int(), 1)

	return nil
}

func pokeAll(x, y int, amplitude float32) {
	for _, p := range panels {
		p.Poke(x, y, amplitude)
	}
}

// stepFn advances one panel and reports what it managed. In equal-time mode
// budget is milliseconds and the step count is the result; in equal-work mode
// budget is a step count and the elapsed time is the result.
func stepFn(_ js.Value, args []js.Value) any {
	i := args[0].Int()
	if i < 0 || i >= len(panels) {
		return nil
	}

	p, b := panels[i], sim.Backends()[i]
	mode, budget := args[1].String(), args[2].Float()

	var (
		steps   int
		elapsed time.Duration
	)

	start := time.Now()

	switch mode {
	case modeEqualWork:
		steps = int(budget)
		p.Step(b, steps)
		elapsed = time.Since(start)
	default: // modeEqualTime
		deadline := time.Duration(budget * float64(time.Millisecond))
		// At least one step always runs, so a budget smaller than a single
		// step still makes progress instead of stalling the panel.
		for {
			p.Step(b, 1)

			steps++
			elapsed = time.Since(start)

			if elapsed >= deadline {
				break
			}
		}
	}

	return map[string]any{
		"steps": steps,
		"ms":    float64(elapsed) / float64(time.Millisecond),
		"total": p.Steps(),
	}
}

// renderFn colours one panel into dst, a Uint8Array view over the caller's
// ImageData. The bytes are copied out rather than exposed as a view into Go's
// memory, which would detach the moment the Go heap grew.
func renderFn(_ js.Value, args []js.Value) any {
	i := args[0].Int()
	if i < 0 || i >= len(panels) {
		return nil
	}

	dst := args[1]
	scale := float32(args[2].Float())

	rgba := make([]byte, panels[i].W*panels[i].H*4)
	panels[i].Render(rgba, scale)
	js.CopyBytesToJS(dst, rgba)

	return nil
}
