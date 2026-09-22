// Package bernstein evaluates planar cubic Bézier curves in Bernstein form
// together with their first and second derivatives.
//
// A curve is defined by four control points P0..P3 and parametrized on
// t in [0,1] as
//
//	r(t) = (1-t)^3 P0 + 3(1-t)^2 t P1 + 3(1-t) t^2 P2 + t^3 P3.
//
// This package is pure math: no I/O, no shared state, safe for concurrent use.
package bernstein

import "math"

// Point is a 2D point or vector.
type Point struct {
	X, Y float64
}

// Add returns p+q.
func (p Point) Add(q Point) Point { return Point{X: p.X + q.X, Y: p.Y + q.Y} }

// Sub returns p-q.
func (p Point) Sub(q Point) Point { return Point{X: p.X - q.X, Y: p.Y - q.Y} }

// Scale returns k*p.
func (p Point) Scale(k float64) Point { return Point{X: p.X * k, Y: p.Y * k} }

// Norm returns the Euclidean length |p|.
func (p Point) Norm() float64 { return math.Hypot(p.X, p.Y) }

// Dot returns the dot product p·q.
func (p Point) Dot(q Point) float64 { return p.X*q.X + p.Y*q.Y }

// Cross returns the scalar (z-component of the) 2D cross product
// p x q = p.X*q.Y - p.Y*q.X.
func (p Point) Cross(q Point) float64 { return p.X*q.Y - p.Y*q.X }

// Curve is a planar cubic Bézier curve with control points P[0]..P[3].
type Curve struct {
	P [4]Point
}

// Eval returns the curve point r(t).
func (c Curve) Eval(t float64) Point {
	u := 1 - t
	b0 := u * u * u
	b1 := 3 * u * u * t
	b2 := 3 * u * t * t
	b3 := t * t * t
	return Point{
		X: b0*c.P[0].X + b1*c.P[1].X + b2*c.P[2].X + b3*c.P[3].X,
		Y: b0*c.P[0].Y + b1*c.P[1].Y + b2*c.P[2].Y + b3*c.P[3].Y,
	}
}

// Deriv returns the first derivative (velocity)
//
//	r'(t) = 3(1-t)^2 (P1-P0) + 6(1-t)t (P2-P1) + 3t^2 (P3-P2).
func (c Curve) Deriv(t float64) Point {
	u := 1 - t
	a0 := 3 * u * u
	a1 := 6 * u * t
	a2 := 3 * t * t
	return Point{
		X: a0*(c.P[1].X-c.P[0].X) + a1*(c.P[2].X-c.P[1].X) + a2*(c.P[3].X-c.P[2].X),
		Y: a0*(c.P[1].Y-c.P[0].Y) + a1*(c.P[2].Y-c.P[1].Y) + a2*(c.P[3].Y-c.P[2].Y),
	}
}

// Deriv2 returns the second derivative (acceleration)
//
//	r''(t) = 6(1-t)(P2 - 2P1 + P0) + 6t(P3 - 2P2 + P1).
func (c Curve) Deriv2(t float64) Point {
	u := 1 - t
	return Point{
		X: 6*u*(c.P[2].X-2*c.P[1].X+c.P[0].X) + 6*t*(c.P[3].X-2*c.P[2].X+c.P[1].X),
		Y: 6*u*(c.P[2].Y-2*c.P[1].Y+c.P[0].Y) + 6*t*(c.P[3].Y-2*c.P[2].Y+c.P[1].Y),
	}
}

// Speed returns |r'(t)|, the integrand of the arc-length integral.
func (c Curve) Speed(t float64) float64 { return c.Deriv(t).Norm() }

// ControlScale returns the longest control-polygon edge, a natural length
// scale of the curve. It is 0 exactly when all four control points coincide.
func (c Curve) ControlScale() float64 {
	s := 0.0
	for i := 0; i < 3; i++ {
		if d := c.P[i+1].Sub(c.P[i]).Norm(); d > s {
			s = d
		}
	}
	return s
}
