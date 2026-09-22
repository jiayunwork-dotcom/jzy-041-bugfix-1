// Package geometry provides curvature and normal-offset computations for
// planar cubic Bézier curves.
//
// Curvature is evaluated in closed form as
//
//	κ(t) = |r'(t) x r''(t)| / |r'(t)|³ = |x'y'' - y'x''| / (x'² + y'²)^{3/2}
//
// where the planar cross product is the scalar x'y” - y'x”. Offset points
// are r(t) + d·n(t) with the unit normal n obtained by rotating the unit
// tangent by +90° (counterclockwise): (x,y) -> (-y,x).
package geometry

import (
	"errors"

	"beziersvc/internal/bernstein"
)

// ErrSingular is returned when the curve has vanishing velocity (a cusp) at
// the queried parameter, where curvature and the normal direction are
// undefined. Callers must surface this as an error, never as Inf/NaN.
var ErrSingular = errors.New("cusp: curve velocity vanishes at this parameter")

// speedEps is the threshold below which a speed is treated as zero, relative
// to the curve's control-polygon scale so it works at any coordinate size.
func speedEps(c bernstein.Curve) float64 {
	const rel = 1e-12
	return rel * c.ControlScale()
}

// Curvature returns κ(t). It returns ErrSingular if |r'(t)| is zero (cusp);
// a straight (collinear) curve is not singular and yields κ = 0.
func Curvature(c bernstein.Curve, t float64) (float64, error) {
	d1 := c.Deriv(t)
	speed := d1.Norm()
	if speed <= speedEps(c) {
		return 0, ErrSingular
	}
	d2 := c.Deriv2(t)
	cross := d1.Cross(d2) // scalar planar cross product x'y'' - y'x''
	return abs(cross) / (speed * speed * speed), nil
}

// UnitTangent returns the unit tangent r'(t)/|r'(t)|, or ErrSingular at a cusp.
func UnitTangent(c bernstein.Curve, t float64) (bernstein.Point, error) {
	d1 := c.Deriv(t)
	speed := d1.Norm()
	if speed <= speedEps(c) {
		return bernstein.Point{}, ErrSingular
	}
	return bernstein.Point{X: d1.X / speed, Y: d1.Y / speed}, nil
}

// UnitNormal returns the unit normal: the unit tangent rotated by +90°
// (counterclockwise), i.e. (x,y) -> (-y,x). ErrSingular at a cusp.
func UnitNormal(c bernstein.Curve, t float64) (bernstein.Point, error) {
	tan, err := UnitTangent(c, t)
	if err != nil {
		return bernstein.Point{}, err
	}
	return bernstein.Point{X: -tan.Y, Y: tan.X}, nil
}

// OffsetPoint returns the normal-equidistant offset point r(t) + d·n(t).
// A negative d offsets to the opposite side. ErrSingular at a cusp.
func OffsetPoint(c bernstein.Curve, t, d float64) (bernstein.Point, error) {
	n, err := UnitNormal(c, t)
	if err != nil {
		return bernstein.Point{}, err
	}
	return c.Eval(t).Add(n.Scale(d)), nil
}

// OffsetPolyline samples the offset curve at segments+1 uniformly spaced
// parameters t_i = i/segments, returning the offset polyline vertices.
// ErrSingular if any sample hits a cusp.
func OffsetPolyline(c bernstein.Curve, d float64, segments int) ([]bernstein.Point, error) {
	pts := make([]bernstein.Point, segments+1)
	for i := 0; i <= segments; i++ {
		t := float64(i) / float64(segments)
		p, err := OffsetPoint(c, t, d)
		if err != nil {
			return nil, err
		}
		pts[i] = p
	}
	return pts, nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
