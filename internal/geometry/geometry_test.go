package geometry_test

import (
	"errors"
	"math"
	"testing"

	"beziersvc/internal/bernstein"
	"beziersvc/internal/geometry"
)

func sampleCurve() bernstein.Curve {
	return bernstein.Curve{P: [4]bernstein.Point{
		{X: 0.3, Y: -1.2}, {X: 2.5, Y: 0.4}, {X: -1.1, Y: 3.3}, {X: 4.0, Y: 0.7},
	}}
}

func lineCurve() bernstein.Curve {
	return bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0},
	}}
}

func cuspCurve() bernstein.Curve {
	// r'(0.5) = 0: the curve runs out and folds back on itself.
	return bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 0}, {X: 1, Y: 0},
	}}
}

func TestCurvatureParabolaClosedForm(t *testing.T) {
	// r(t) = (t, t²): κ(t) = 2 / (1+4t²)^{3/2}.
	c := bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1.0 / 3, Y: 0}, {X: 2.0 / 3, Y: 1.0 / 3}, {X: 1, Y: 1},
	}}
	for _, te := range []float64{0, 0.2, 0.5, 0.9, 1} {
		want := 2 / math.Pow(1+4*te*te, 1.5)
		got, err := geometry.Curvature(c, te)
		if err != nil {
			t.Fatalf("Curvature(%v) error: %v", te, err)
		}
		if math.Abs(got-want) > 1e-12 {
			t.Fatalf("κ(%v) = %v, want %v", te, got, want)
		}
	}
}

func TestCurvatureCollinearIsZero(t *testing.T) {
	// Collinear but NOT evenly spaced: r'' ≠ 0 yet parallel to r'.
	c := bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 0.5, Y: 0}, {X: 2.5, Y: 0}, {X: 3, Y: 0},
	}}
	for i := 0; i <= 10; i++ {
		te := float64(i) / 10
		k, err := geometry.Curvature(c, te)
		if err != nil {
			t.Fatalf("Curvature(%v) on a line must not error: %v", te, err)
		}
		if k > 1e-12 {
			t.Fatalf("κ(%v) = %v on a straight line, want 0", te, k)
		}
	}
}

func TestCurvatureCuspRejected(t *testing.T) {
	if _, err := geometry.Curvature(cuspCurve(), 0.5); !errors.Is(err, geometry.ErrSingular) {
		t.Fatalf("cusp curvature: err = %v, want ErrSingular", err)
	}
	deg := bernstein.Curve{P: [4]bernstein.Point{{X: 1, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 1}}}
	if _, err := geometry.Curvature(deg, 0.3); !errors.Is(err, geometry.ErrSingular) {
		t.Fatalf("coincident-points curvature: err = %v, want ErrSingular", err)
	}
}

func TestCurvatureTranslationInvariant(t *testing.T) {
	c := sampleCurve()
	shift := bernstein.Point{X: 37.25, Y: -11.75}
	var moved bernstein.Curve
	for i, p := range c.P {
		moved.P[i] = p.Add(shift)
	}
	for i := 0; i <= 8; i++ {
		te := float64(i) / 8
		k0, err0 := geometry.Curvature(c, te)
		k1, err1 := geometry.Curvature(moved, te)
		if err0 != nil || err1 != nil {
			t.Fatalf("unexpected errors: %v %v", err0, err1)
		}
		if math.Abs(k0-k1) > 1e-9*math.Max(1, k0) {
			t.Fatalf("κ changed under translation at t=%v: %v vs %v", te, k0, k1)
		}
	}
}

func TestCurvatureScalesAsInverseK(t *testing.T) {
	c := sampleCurve()
	for _, k := range []float64{2.5, -2, 0.1} {
		var scaled bernstein.Curve
		for i, p := range c.P {
			scaled.P[i] = p.Scale(k)
		}
		for _, te := range []float64{0.15, 0.5, 0.85} {
			k0, _ := geometry.Curvature(c, te)
			k1, _ := geometry.Curvature(scaled, te)
			want := k0 / math.Abs(k)
			if math.Abs(k1-want) > 1e-9*math.Max(1, want) {
				t.Fatalf("scale %v: κ(%v) = %v, want %v", k, te, k1, want)
			}
		}
	}
}

func TestOffsetZeroDistanceCoincides(t *testing.T) {
	c := sampleCurve()
	pts, err := geometry.OffsetPolyline(c, 0, 32)
	if err != nil {
		t.Fatalf("offset d=0: %v", err)
	}
	for i, p := range pts {
		want := c.Eval(float64(i) / 32)
		if d := p.Sub(want).Norm(); d > 1e-12 {
			t.Fatalf("offset point %d = %v, want %v (d=0)", i, p, want)
		}
	}
}

func TestOffsetCollinearIsParallelLine(t *testing.T) {
	c := lineCurve()
	for _, d := range []float64{2, -0.5} {
		pts, err := geometry.OffsetPolyline(c, d, 8)
		if err != nil {
			t.Fatalf("offset of a line: %v", err)
		}
		for i, p := range pts {
			// Distance from the original line (the x-axis) must be exactly |d|.
			if math.Abs(math.Abs(p.Y)-math.Abs(d)) > 1e-12 {
				t.Fatalf("offset point %d y=%v, distance to line != |d|=%v", i, p.Y, math.Abs(d))
			}
			// Same side of the line everywhere.
			if p.Y*d < 0 {
				t.Fatalf("offset point %d flipped side: y=%v for d=%v", i, p.Y, d)
			}
			// The offset of a straight line stays straight: x matches the curve.
			want := c.Eval(float64(i) / 8)
			if math.Abs(p.X-want.X) > 1e-12 {
				t.Fatalf("offset point %d x=%v, want %v", i, p.X, want.X)
			}
		}
	}
}

func TestOffsetCuspRejected(t *testing.T) {
	if _, err := geometry.OffsetPolyline(cuspCurve(), 0.1, 16); !errors.Is(err, geometry.ErrSingular) {
		t.Fatalf("cusp offset: err = %v, want ErrSingular", err)
	}
}

func TestNormalIsUnitAndPerpendicular(t *testing.T) {
	c := sampleCurve()
	for _, te := range []float64{0.1, 0.5, 0.9} {
		n, err := geometry.UnitNormal(c, te)
		if err != nil {
			t.Fatalf("UnitNormal(%v): %v", te, err)
		}
		if math.Abs(n.Norm()-1) > 1e-12 {
			t.Fatalf("|n(%v)| = %v, want 1", te, n.Norm())
		}
		tan, _ := geometry.UnitTangent(c, te)
		if math.Abs(n.Dot(tan)) > 1e-12 {
			t.Fatalf("n·t(%v) = %v, want 0", te, n.Dot(tan))
		}
	}
}
