// Package sim holds the demo's simulation state: the physics and the buffers,
// kept out of the wasm-only glue so `go test` can exercise them natively.
package sim

import (
	"math"

	"github.com/MeKo-Christian/simd-wasm/simd"
)

// Backend selects which implementation of a kernel a step runs on. All three
// live in one binary, so the demo can race them against identical state.
type Backend int

const (
	// BackendGo is the pure-Go loop compiled by the stock Go wasm backend.
	BackendGo Backend = iota
	// BackendC is the clang-compiled kernel without simd128.
	BackendC
	// BackendSIMD is the clang-compiled kernel with simd128.
	BackendSIMD
)

// String returns the label the demo shows above each panel.
func (b Backend) String() string {
	switch b {
	case BackendGo:
		return "Go"
	case BackendC:
		return "C"
	case BackendSIMD:
		return "C + SIMD"
	default:
		return "unknown"
	}
}

// Backends returns every backend, in display order.
func Backends() []Backend { return []Backend{BackendGo, BackendC, BackendSIMD} }

// Wave is a 2D explicit finite-difference wave equation on a w*h grid: a pond
// you can drop stones into. The stencil is the 5-point discrete Laplacian, the
// float32 counterpart of algo-pde's fd.Apply2D, wrapped in a leapfrog time
// step. Borders are held at zero (Dirichlet), so waves reflect off the walls.
type Wave struct {
	W, H int

	// c2 is (c*dt/dx)^2, the squared Courant number. Explicit leapfrog is
	// stable for c2 <= 0.5 in 2D; the default sits below that.
	c2   float32
	damp float32

	cur, prev, next []float32
	steps           int
	drivers         []driver
}

// A driver is a cell held oscillating: a loudspeaker in the pond. Without one,
// a damped pond is only interesting for as long as the last splash takes to
// die -- and the fastest backend, being furthest ahead, would go flat first,
// which is exactly backwards for a demo about speed.
//
// It pulses rather than running continuously, because a continuous drive makes
// every panel converge on the same steady interference pattern: they would all
// look alike no matter how far ahead the fast ones were. Pulsed, each panel
// shows an expanding ring whose radius is a direct readout of how many steps
// it has taken.
//
// Everything here depends only on the step count, so the backends stay
// comparable: at equal steps they are driven identically.
type driver struct {
	idx        int
	freq, ampl float32
	// burst steps of emission out of every period; period <= 0 drives
	// continuously.
	burst, period int
}

// Default simulation constants. c2 is comfortably inside the 2D stability
// limit and damp bleeds energy slowly enough that ripples persist but a long
// unattended run still settles instead of ringing forever.
const (
	defaultC2   = 0.4
	defaultDamp = 0.9998
)

// NewWave allocates a w*h pond. Dimensions below 3 leave nothing to step, so
// they are clamped.
func NewWave(w, h int) *Wave {
	w, h = max(w, 3), max(h, 3)

	return &Wave{
		W: w, H: h,
		c2:   defaultC2,
		damp: defaultDamp,
		cur:  make([]float32, w*h),
		prev: make([]float32, w*h),
		next: make([]float32, w*h),
	}
}

// Steps reports how many steps have been taken since the last Reset.
func (s *Wave) Steps() int { return s.steps }

// Field exposes the current field, for tests and for rendering.
func (s *Wave) Field() []float32 { return s.cur }

// Reset clears the pond back to flat water, keeping the drivers.
func (s *Wave) Reset() {
	clear(s.cur)
	clear(s.prev)
	clear(s.next)

	s.steps = 0
}

// Drive adds a pulsed oscillating source at (x, y). freq is in cycles per
// step, so the wavelength it radiates follows from freq and the Courant
// number; burst and period set the emission gate.
func (s *Wave) Drive(x, y int, freq, ampl float32, burst, period int) {
	if x < 1 || y < 1 || x >= s.W-1 || y >= s.H-1 {
		return
	}

	s.drivers = append(s.drivers, driver{
		idx: y*s.W + x, freq: freq, ampl: ampl, burst: burst, period: period,
	})
}

// Poke drops a stone at (x, y): a smooth Gaussian bump added to the current
// field, wide enough to be resolved by the grid rather than ringing as a
// single-cell spike.
func (s *Wave) Poke(x, y int, amplitude float32) {
	const radius = 6

	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			px, py := x+dx, y+dy
			if px < 1 || py < 1 || px >= s.W-1 || py >= s.H-1 {
				continue
			}

			r2 := float32(dx*dx + dy*dy)
			if r2 > radius*radius {
				continue
			}

			// exp(-r^2/8) without math.Exp: a cheap smooth bump is enough,
			// and this keeps the hot demo path allocation- and libm-free.
			f := 1 - r2/(radius*radius)
			s.cur[py*s.W+px] += amplitude * f * f
		}
	}
}

// Step advances n steps on the given backend and returns the number taken.
func (s *Wave) Step(b Backend, n int) int {
	for range n {
		s.stepOnce(b)
	}

	return n
}

// Render writes the current field into rgba, which must hold W*H pixels. The
// colormap always runs on the fastest available backend and is never part of
// the measurement -- every panel renders identically, so the comparison stays
// about the stepper.
func (s *Wave) Render(rgba []byte, scale float32) {
	simd.Colormap(rgba, s.cur, scale)
}

func (s *Wave) stepOnce(b Backend) {
	switch b {
	case BackendSIMD:
		simd.WaveStep(s.next, s.cur, s.prev, s.W, s.H, s.c2, s.damp)
	case BackendC:
		simd.CWaveStep(s.next, s.cur, s.prev, s.W, s.H, s.c2, s.damp)
	case BackendGo:
		simd.ScalarWaveStep(s.next, s.cur, s.prev, s.W, s.H, s.c2, s.damp)
	default:
		simd.ScalarWaveStep(s.next, s.cur, s.prev, s.W, s.H, s.c2, s.damp)
	}

	// Rotate the three buffers instead of copying: next becomes cur, cur
	// becomes prev, and the old prev is reused as scratch for the next step.
	s.prev, s.cur, s.next = s.cur, s.next, s.prev
	s.steps++

	for _, d := range s.drivers {
		if d.period > 0 && s.steps%d.period >= d.burst {
			continue
		}

		phase := 2 * math.Pi * float64(d.freq) * float64(s.steps)
		s.cur[d.idx] += d.ampl * float32(math.Sin(phase))
	}
}
