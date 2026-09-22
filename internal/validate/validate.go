// Package validate checks incoming requests and reports failures as
// structured, typed errors — never panics, never silent empty results.
package validate

import (
	"fmt"
	"math"

	"beziersvc/internal/bernstein"
)

// Type classifies a request failure for the structured error response.
type Type string

const (
	// TypeInvalidInput: malformed or out-of-domain input (wrong point count,
	// missing/non-numeric coordinates, non-positive tolerance, ...).
	TypeInvalidInput Type = "invalid_input"
	// TypeSingular: the input is well-formed but the requested quantity is
	// undefined there (curvature/normal at a cusp).
	TypeSingular Type = "singular"
	// TypeNotFound: unknown route.
	TypeNotFound Type = "not_found"
	// TypeMethodNotAllowed: known route, wrong HTTP method.
	TypeMethodNotAllowed Type = "method_not_allowed"
	// TypeInternal: unexpected server-side failure (safety net only).
	TypeInternal Type = "internal"
)

// Error is a structured, typed failure carried through the whole stack.
type Error struct {
	Type    Type   `json:"type"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s (field: %s)", e.Type, e.Message, e.Field)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Invalid builds an invalid_input error for a specific field.
func Invalid(field, format string, args ...any) *Error {
	return &Error{Type: TypeInvalidInput, Field: field, Message: fmt.Sprintf(format, args...)}
}

// Singular builds a singular error (e.g. curvature at a cusp).
func Singular(format string, args ...any) *Error {
	return &Error{Type: TypeSingular, Message: fmt.Sprintf(format, args...)}
}

// PointInput is the raw JSON contract of one control point. Pointer fields
// let validation distinguish "missing" from a legitimate numeric zero.
type PointInput struct {
	X *float64 `json:"x"`
	Y *float64 `json:"y"`
}

// MaxCoordinate bounds |x| and |y| so downstream squares and cubes of
// coordinates can never overflow to +Inf.
const MaxCoordinate = 1e12

// ControlPoints validates exactly four finite control points and returns
// them as a Bernstein curve.
func ControlPoints(pts []PointInput) (bernstein.Curve, *Error) {
	var c bernstein.Curve
	if len(pts) != 4 {
		return c, Invalid("points", "a cubic Bézier curve needs exactly 4 control points, got %d", len(pts))
	}
	for i, p := range pts {
		x, err := coord(p.X, fmt.Sprintf("points[%d].x", i))
		if err != nil {
			return c, err
		}
		y, err := coord(p.Y, fmt.Sprintf("points[%d].y", i))
		if err != nil {
			return c, err
		}
		c.P[i] = bernstein.Point{X: x, Y: y}
	}
	return c, nil
}

func coord(v *float64, field string) (float64, *Error) {
	if v == nil {
		return 0, Invalid(field, "coordinate is missing")
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) {
		return 0, Invalid(field, "coordinate must be a finite number")
	}
	if math.Abs(*v) > MaxCoordinate {
		return 0, Invalid(field, "coordinate magnitude exceeds the limit %g", MaxCoordinate)
	}
	return *v, nil
}

// Tolerance validates an optional arc-length tolerance, substituting def
// when the request leaves it unset. Zero and negative values are rejected.
func Tolerance(v *float64, def float64) (float64, *Error) {
	if v == nil {
		return def, nil
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) || *v <= 0 {
		return 0, Invalid("tolerance", "tolerance must be a positive finite number, got %v", *v)
	}
	return *v, nil
}

// ParamT validates a curve parameter: it must lie in [0,1].
func ParamT(t float64, field string) *Error {
	if math.IsNaN(t) || math.IsInf(t, 0) || t < 0 || t > 1 {
		return Invalid(field, "parameter t must lie in [0,1], got %v", t)
	}
	return nil
}

// Distance validates a required offset distance (any finite value; the sign
// only selects the offset side).
func Distance(v *float64) (float64, *Error) {
	if v == nil {
		return 0, Invalid("offset.distance", "offset distance is required")
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) || math.Abs(*v) > MaxCoordinate {
		return 0, Invalid("offset.distance", "distance must be finite with |d| <= %g", MaxCoordinate)
	}
	return *v, nil
}

// Segments validates an optional polyline segment count.
func Segments(v *int, def, max int) (int, *Error) {
	if v == nil {
		return def, nil
	}
	if *v < 1 || *v > max {
		return 0, Invalid("offset.segments", "segments must be in [1,%d], got %d", max, *v)
	}
	return *v, nil
}
