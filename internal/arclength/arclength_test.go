package arclength_test

import (
	"math"
	"testing"

	"beziersvc/internal/arclength"
	"beziersvc/internal/bernstein"
)

// bow is a clearly bent arc: chord 3, control polygon much longer.
func bow() bernstein.Curve {
	return bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1, Y: 2}, {X: 2, Y: 2}, {X: 3, Y: 0},
	}}
}

// chordal approximates arc length by the n-segment inscribed polygon —
// the classic estimator that systematically underestimates bent curves.
func chordal(c bernstein.Curve, n int) float64 {
	L := 0.0
	prev := c.Eval(0)
	for i := 1; i <= n; i++ {
		p := c.Eval(float64(i) / float64(n))
		L += p.Sub(prev).Norm()
		prev = p
	}
	return L
}

func TestStraightLineLengthIsExact(t *testing.T) {
	// Evenly spaced points on the 3-4-5 line: length must be exactly 5.
	c := bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1, Y: 4.0 / 3}, {X: 2, Y: 8.0 / 3}, {X: 3, Y: 4},
	}}
	res := arclength.Length(c, 1e-12, arclength.DefaultLimits)
	if !res.Converged {
		t.Fatal("straight line did not converge")
	}
	if math.Abs(res.Length-5) > 1e-12 {
		t.Fatalf("length = %v, want 5", res.Length)
	}
	if res.Residual != 0 {
		t.Fatalf("residual = %v, want 0 for a straight line", res.Residual)
	}
}

func TestParabolaLengthMatchesClosedForm(t *testing.T) {
	// r(t) = (t, t²): ∫₀¹ √(1+4t²) dt = √5/2 + asinh(2)/4.
	want := math.Sqrt(5)/2 + math.Asinh(2)/4 // ≈ 1.4789428575445974
	c := bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1.0 / 3, Y: 0}, {X: 2.0 / 3, Y: 1.0 / 3}, {X: 1, Y: 1},
	}}
	res := arclength.Length(c, 1e-11, arclength.DefaultLimits)
	if !res.Converged {
		t.Fatal("parabola did not converge")
	}
	if math.Abs(res.Length-want) > 1e-9 {
		t.Fatalf("length = %.15f, want %.15f", res.Length, want)
	}
	if res.Residual > 1e-11 {
		t.Fatalf("residual %v exceeds requested tolerance", res.Residual)
	}
}

func TestRefinementConvergesAndBeatsChord(t *testing.T) {
	c := bow()
	chord := c.P[3].Sub(c.P[0]).Norm() // 3
	polygon := 0.0
	for i := 0; i < 3; i++ {
		polygon += c.P[i+1].Sub(c.P[i]).Norm()
	}

	coarse := arclength.Length(c, 1e-1, arclength.DefaultLimits)
	fine := arclength.Length(c, 1e-12, arclength.DefaultLimits)
	if !fine.Converged {
		t.Fatal("fine integration did not converge")
	}
	// Refinement must actually subdivide further for tighter tolerances.
	if fine.Segments <= coarse.Segments {
		t.Fatalf("fine segments %d <= coarse segments %d", fine.Segments, coarse.Segments)
	}
	// The converged value sits strictly between chord and control polygon.
	if fine.Length <= chord {
		t.Fatalf("length %v not greater than chord %v", fine.Length, chord)
	}
	if fine.Length >= polygon {
		t.Fatalf("length %v not smaller than control polygon %v", fine.Length, polygon)
	}
	// The coarse value is a measurably worse approximation of the limit:
	// it has not converged even at the 1e-8 level.
	if d := math.Abs(coarse.Length - fine.Length); d < 1e-8 {
		t.Fatalf("coarse and fine lengths suspiciously close: %v vs %v", coarse.Length, fine.Length)
	}
	// Residual honestly reported and within tolerance once converged.
	if fine.Residual > 1e-12 {
		t.Fatalf("residual %v exceeds tolerance", fine.Residual)
	}
}

func TestChordalUnderestimatesAndApproachesFromBelow(t *testing.T) {
	c := bow()
	c2, c16 := chordal(c, 2), chordal(c, 16)
	fine := arclength.Length(c, 1e-12, arclength.DefaultLimits)
	if !(c2 < c16) {
		t.Fatalf("chordal estimate did not increase under refinement: %v !< %v", c2, c16)
	}
	if c16 > fine.Length+1e-12 {
		t.Fatalf("chordal estimate %v overshoots converged length %v", c16, fine.Length)
	}
	if fine.Length-c16 > 1e-2 {
		t.Fatalf("16-segment chordal %v too far from converged %v", c16, fine.Length)
	}
}

func TestDegenerateCurveHasZeroLength(t *testing.T) {
	c := bernstein.Curve{P: [4]bernstein.Point{
		{X: 1.5, Y: -2.5}, {X: 1.5, Y: -2.5}, {X: 1.5, Y: -2.5}, {X: 1.5, Y: -2.5},
	}}
	res := arclength.Length(c, 1e-9, arclength.DefaultLimits)
	if !res.Converged || res.Length != 0 {
		t.Fatalf("degenerate curve: length=%v converged=%v, want 0/true", res.Length, res.Converged)
	}
}

func TestCuspCurveStillHasHonestLength(t *testing.T) {
	// Cusps break curvature, not arc length: the integral must still converge.
	c := bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 0}, {X: 1, Y: 0},
	}}
	res := arclength.Length(c, 1e-10, arclength.DefaultLimits)
	if !res.Converged {
		t.Fatal("cusp curve arc length did not converge")
	}
	if res.Length <= 0 || math.IsNaN(res.Length) || math.IsInf(res.Length, 0) {
		t.Fatalf("bad length %v for cusp curve", res.Length)
	}
}
