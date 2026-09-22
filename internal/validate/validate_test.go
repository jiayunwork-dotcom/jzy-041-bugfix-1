package validate_test

import (
	"math"
	"testing"

	"beziersvc/internal/validate"
)

func f64(v float64) *float64 { return &v }

func TestControlPointsRejectsWrongCount(t *testing.T) {
	pts := []validate.PointInput{{X: f64(0), Y: f64(0)}, {X: f64(1), Y: f64(1)}, {X: f64(2), Y: f64(0)}}
	if _, err := validate.ControlPoints(pts); err == nil || err.Type != validate.TypeInvalidInput {
		t.Fatalf("3 points: err = %v, want invalid_input", err)
	}
	five := append(pts, pts[0], pts[1])
	if _, err := validate.ControlPoints(five); err == nil || err.Type != validate.TypeInvalidInput {
		t.Fatalf("5 points: err = %v, want invalid_input", err)
	}
}

func TestControlPointsRejectsMissingCoordinate(t *testing.T) {
	pts := []validate.PointInput{
		{X: f64(0), Y: f64(0)}, {X: f64(1), Y: f64(1)},
		{X: f64(2)}, {X: f64(3), Y: f64(0)},
	}
	_, err := validate.ControlPoints(pts)
	if err == nil || err.Type != validate.TypeInvalidInput {
		t.Fatalf("missing y: err = %v, want invalid_input", err)
	}
	if err.Field != "points[2].y" {
		t.Fatalf("error field = %q, want points[2].y", err.Field)
	}
}

func TestControlPointsRejectsNonFiniteAndHuge(t *testing.T) {
	bad := []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1e13}
	for _, v := range bad {
		pts := []validate.PointInput{
			{X: f64(0), Y: f64(0)}, {X: f64(v), Y: f64(1)},
			{X: f64(2), Y: f64(0)}, {X: f64(3), Y: f64(0)},
		}
		if _, err := validate.ControlPoints(pts); err == nil || err.Type != validate.TypeInvalidInput {
			t.Fatalf("coordinate %v: err = %v, want invalid_input", v, err)
		}
	}
}

func TestControlPointsAcceptsZeros(t *testing.T) {
	pts := []validate.PointInput{
		{X: f64(0), Y: f64(0)}, {X: f64(0), Y: f64(0)},
		{X: f64(0), Y: f64(0)}, {X: f64(0), Y: f64(0)},
	}
	if _, err := validate.ControlPoints(pts); err != nil {
		t.Fatalf("all-zero points are valid input (a degenerate curve), got %v", err)
	}
}

func TestToleranceValidation(t *testing.T) {
	if got, err := validate.Tolerance(nil, 1e-9); err != nil || got != 1e-9 {
		t.Fatalf("default tolerance: got %v err %v", got, err)
	}
	for _, v := range []float64{0, -1e-3, math.NaN(), math.Inf(1)} {
		if _, err := validate.Tolerance(f64(v), 1e-9); err == nil || err.Type != validate.TypeInvalidInput {
			t.Fatalf("tolerance %v: err = %v, want invalid_input", v, err)
		}
	}
	if got, err := validate.Tolerance(f64(1e-6), 1e-9); err != nil || got != 1e-6 {
		t.Fatalf("tolerance 1e-6: got %v err %v", got, err)
	}
}

func TestParamTValidation(t *testing.T) {
	for _, v := range []float64{-0.1, 1.0001, math.NaN()} {
		if err := validate.ParamT(v, "t"); err == nil || err.Type != validate.TypeInvalidInput {
			t.Fatalf("t=%v: err = %v, want invalid_input", v, err)
		}
	}
	for _, v := range []float64{0, 0.5, 1} {
		if err := validate.ParamT(v, "t"); err != nil {
			t.Fatalf("t=%v should be valid: %v", v, err)
		}
	}
}

func TestDistanceAndSegments(t *testing.T) {
	if _, err := validate.Distance(nil); err == nil {
		t.Fatal("missing distance must be rejected")
	}
	if got, err := validate.Distance(f64(-3.5)); err != nil || got != -3.5 {
		t.Fatalf("negative distance is legal (other side): got %v err %v", got, err)
	}
	if _, err := validate.Segments(nil, 64, 4096); err != nil {
		t.Fatalf("default segments: %v", err)
	}
	n := 0
	if _, err := validate.Segments(&n, 64, 4096); err == nil {
		t.Fatal("segments=0 must be rejected")
	}
	n = 5000
	if _, err := validate.Segments(&n, 64, 4096); err == nil {
		t.Fatal("segments above max must be rejected")
	}
}
