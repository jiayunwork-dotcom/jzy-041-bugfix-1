// Package arclength computes the arc length of a planar cubic Bézier curve
// by adaptive numerical integration of the speed |r'(t)| over [0,1].
//
// The integrator never falls back to the control-polygon perimeter or any
// other fixed-resolution shortcut: every panel of the partition is halved
// until one more refinement changes that panel's integral by less than its
// allocated share of the requested tolerance. The returned Result honestly
// reports how many panels were finally used and how large the residual
// (the summed last-refinement change) is.
package arclength

import (
	"math"

	"beziersvc/internal/bernstein"
)

// Limits bounds the adaptive refinement.
type Limits struct {
	// MinDepth is the number of halvings every panel must undergo before it
	// may be accepted (guards against accidental early acceptance).
	MinDepth int
	// MaxDepth caps the halving depth of any panel (the refinement ceiling).
	MaxDepth int
}

// DefaultLimits are the service defaults: at least 2 and at most 20 halvings.
var DefaultLimits = Limits{MinDepth: 2, MaxDepth: 20}

// Result reports the outcome of an adaptive arc-length integration.
type Result struct {
	// Length is the converged estimate of ∫₀¹ |r'(t)| dt.
	Length float64
	// Segments is the number of panels [0,1] was finally split into.
	Segments int
	// Residual is the sum, over all accepted panels, of the absolute change
	// produced by the last refinement of that panel. When Converged is true
	// the residual is guaranteed to be <= the requested tolerance.
	Residual float64
	// Converged is false only if the MaxDepth ceiling was hit somewhere.
	Converged bool
}

// Length integrates the speed of c over [0,1] to the requested tolerance
// using adaptive Simpson refinement. tol must be positive; non-positive
// values are defensively clamped (the HTTP layer rejects them earlier).
func Length(c bernstein.Curve, tol float64, lim Limits) Result {
	if tol <= 0 || math.IsNaN(tol) || math.IsInf(tol, 0) {
		tol = 1e-15
	}
	if lim.MaxDepth < 1 {
		lim.MaxDepth = DefaultLimits.MaxDepth
	}
	if lim.MinDepth < 0 {
		lim.MinDepth = 0
	}
	if lim.MinDepth > lim.MaxDepth {
		lim.MinDepth = lim.MaxDepth
	}

	f := c.Speed
	fa, fm, fb := f(0), f(0.5), f(1)
	whole := simpson(0, 1, fa, fm, fb)

	r := &Result{Converged: true}
	r.Length = r.refine(f, 0, 0.5, 1, fa, fm, fb, whole, tol, 0, lim)
	return *r
}

// simpson returns Simpson's estimate of ∫ₐᵇ f given the endpoint and
// midpoint values.
func simpson(a, b, fa, fm, fb float64) float64 {
	return (b - a) / 6 * (fa + 4*fm + fb)
}

// refine compares Simpson's estimate on [a,b] with the sum of the estimates
// on its two halves. If one more refinement changes the value by at most tol
// (this panel's share of the global tolerance), the panel is accepted with
// Richardson extrapolation; otherwise both halves are refined recursively
// with halved tolerance shares.
func (r *Result) refine(f func(float64) float64, a, m, b, fa, fm, fb, whole, tol float64, depth int, lim Limits) float64 {
	lm, rm := 0.5*(a+m), 0.5*(m+b)
	flm, frm := f(lm), f(rm)
	left := simpson(a, m, fa, flm, fm)
	right := simpson(m, b, fm, frm, fb)

	delta := left + right - whole
	if depth >= lim.MinDepth && math.Abs(delta) <= tol {
		r.Segments += 2
		r.Residual += math.Abs(delta)
		return left + right + delta/15 // Richardson extrapolation
	}
	if depth >= lim.MaxDepth {
		r.Segments += 2
		r.Residual += math.Abs(delta)
		r.Converged = false
		return left + right + delta/15
	}
	half := 0.5 * tol
	return r.refine(f, a, lm, m, fa, flm, fm, left, half, depth+1, lim) +
		r.refine(f, m, rm, b, fm, frm, fb, right, half, depth+1, lim)
}
