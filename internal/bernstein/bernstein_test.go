package bernstein_test

import (
	"math"
	"testing"

	"beziersvc/internal/bernstein"
)

func almostEq(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// parabola returns the cubic Bézier that exactly traces r(t) = (t, t²):
// control x are evenly spaced (so x(t)=t) and control y are 0,0,1/3,1.
func parabola() bernstein.Curve {
	return bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1.0 / 3, Y: 0}, {X: 2.0 / 3, Y: 1.0 / 3}, {X: 1, Y: 1},
	}}
}

func TestEvalEndpoints(t *testing.T) {
	c := bernstein.Curve{P: [4]bernstein.Point{
		{X: 1, Y: 2}, {X: 3, Y: 5}, {X: 7, Y: 11}, {X: 13, Y: 17},
	}}
	if got := c.Eval(0); got != c.P[0] {
		t.Fatalf("r(0) = %v, want %v", got, c.P[0])
	}
	if got := c.Eval(1); got != c.P[3] {
		t.Fatalf("r(1) = %v, want %v", got, c.P[3])
	}
}

func TestEvalMidpoint(t *testing.T) {
	// Bow curve (0,0),(0,1),(1,1),(1,0): Bernstein weights at t=0.5 are
	// (1/8, 3/8, 3/8, 1/8), giving r(0.5) = (0.5, 0.75).
	c := bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 0},
	}}
	got := c.Eval(0.5)
	if !almostEq(got.X, 0.5, 1e-15) || !almostEq(got.Y, 0.75, 1e-15) {
		t.Fatalf("r(0.5) = %v, want (0.5, 0.75)", got)
	}
}

func TestParabolaParametrization(t *testing.T) {
	c := parabola()
	for _, te := range []float64{0, 0.1, 0.37, 0.5, 0.83, 1} {
		got := c.Eval(te)
		if !almostEq(got.X, te, 1e-12) || !almostEq(got.Y, te*te, 1e-12) {
			t.Fatalf("r(%v) = %v, want (%v, %v)", te, got, te, te*te)
		}
	}
}

func TestDerivativesMatchFiniteDifferences(t *testing.T) {
	c := bernstein.Curve{P: [4]bernstein.Point{
		{X: 0.3, Y: -1.2}, {X: 2.5, Y: 0.4}, {X: -1.1, Y: 3.3}, {X: 4.0, Y: 0.7},
	}}
	const h = 1e-6
	for _, te := range []float64{0.05, 0.31, 0.5, 0.77, 0.95} {
		d1 := c.Deriv(te)
		fd1x := (c.Eval(te+h).X - c.Eval(te-h).X) / (2 * h)
		fd1y := (c.Eval(te+h).Y - c.Eval(te-h).Y) / (2 * h)
		if !almostEq(d1.X, fd1x, 1e-6) || !almostEq(d1.Y, fd1y, 1e-6) {
			t.Fatalf("r'(%v) = %v, finite difference gives (%v, %v)", te, d1, fd1x, fd1y)
		}
		d2 := c.Deriv2(te)
		fd2x := (c.Deriv(te+h).X - c.Deriv(te-h).X) / (2 * h)
		fd2y := (c.Deriv(te+h).Y - c.Deriv(te-h).Y) / (2 * h)
		if !almostEq(d2.X, fd2x, 1e-5) || !almostEq(d2.Y, fd2y, 1e-5) {
			t.Fatalf("r''(%v) = %v, finite difference gives (%v, %v)", te, d2, fd2x, fd2y)
		}
	}
}

func TestControlScale(t *testing.T) {
	c := bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 3, Y: 4}, {X: 3.5, Y: 4}, {X: 4, Y: 4},
	}}
	if got := c.ControlScale(); !almostEq(got, 5, 1e-15) {
		t.Fatalf("ControlScale = %v, want 5", got)
	}
	deg := bernstein.Curve{P: [4]bernstein.Point{{X: 2, Y: 2}, {X: 2, Y: 2}, {X: 2, Y: 2}, {X: 2, Y: 2}}}
	if got := deg.ControlScale(); got != 0 {
		t.Fatalf("ControlScale of coincident points = %v, want 0", got)
	}
}
