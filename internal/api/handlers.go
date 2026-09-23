package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"beziersvc/internal/arclength"
	"beziersvc/internal/bernstein"
	"beziersvc/internal/geometry"
	"beziersvc/internal/validate"
)

// demoCurve is the preloaded bow-shaped cubic Bézier. Its arc length is
// exactly 2 while its chord |P3-P0| is exactly 1, so the example makes the
// direction and magnitude of the arc-length integral obvious at a glance.
var demoCurve = bernstein.Curve{P: [4]bernstein.Point{
	{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 0},
}}

// bind decodes a JSON body, mapping decoding failures (including non-numeric
// coordinates) to typed invalid_input errors.
func bind(c *gin.Context, v any) *validate.Error {
	if err := c.ShouldBindJSON(v); err != nil {
		var ute *json.UnmarshalTypeError
		if errors.As(err, &ute) {
			return validate.Invalid(ute.Field, "expected a number, got %s", ute.Value)
		}
		return validate.Invalid("", "malformed JSON body: %v", err)
	}
	return nil
}

// arcLengthOf runs the shared adaptive integration for one curve.
func (s *Server) arcLengthOf(curve bernstein.Curve, tol float64) *ArcLengthResponse {
	res := arclength.Length(curve, tol, s.limits())
	return &ArcLengthResponse{
		Length:    res.Length,
		Segments:  res.Segments,
		Residual:  res.Residual,
		Converged: res.Converged,
		Tolerance: tol,
	}
}

// curvaturesOf evaluates the shared closed-form curvature at each t.
func (s *Server) curvaturesOf(curve bernstein.Curve, ts []float64, field string) ([]CurvatureValue, *validate.Error) {
	if len(ts) > s.cfg.MaxCurvaturePoints {
		return nil, validate.Invalid(field, "at most %d parameters per curve, got %d", s.cfg.MaxCurvaturePoints, len(ts))
	}
	out := make([]CurvatureValue, 0, len(ts))
	for i, t := range ts {
		if verr := validate.ParamT(t, fmt.Sprintf("%s[%d]", field, i)); verr != nil {
			return nil, verr
		}
		k, err := geometry.Curvature(curve, t)
		if err != nil {
			return nil, validate.Singular("curvature undefined at t=%v: cusp (zero velocity)", t)
		}
		out = append(out, CurvatureValue{T: t, Curvature: k})
	}
	return out, nil
}

// offsetOf builds the shared normal-offset polyline.
func (s *Server) offsetOf(curve bernstein.Curve, spec *OffsetSpec) (*OffsetResponse, *validate.Error) {
	d, verr := validate.Distance(spec.Distance)
	if verr != nil {
		return nil, verr
	}
	seg, verr := validate.Segments(spec.Segments, s.cfg.DefaultOffsetSegments, s.cfg.MaxOffsetSegments)
	if verr != nil {
		return nil, verr
	}
	pts, err := geometry.OffsetPolyline(curve, d, seg)
	if err != nil {
		return nil, validate.Singular("offset undefined: cusp (zero velocity) on curve")
	}
	out := make([]PointOut, len(pts))
	for i, p := range pts {
		out[i] = PointOut{X: p.X, Y: p.Y}
	}
	return &OffsetResponse{Distance: d, Segments: seg, Polyline: out}, nil
}

// computeAll is the single evaluation pipeline shared by /evaluate and
// /batch, so a curve can never yield different numbers through the two
// endpoints. Arc length is always computed; curvature and offset on demand.
func (s *Server) computeAll(points []validate.PointInput, tolPtr *float64, ts []float64, off *OffsetSpec) (*EvaluateResponse, *validate.Error) {
	curve, verr := validate.ControlPoints(points)
	if verr != nil {
		return nil, verr
	}
	tol, verr := validate.Tolerance(tolPtr, s.cfg.DefaultTolerance)
	if verr != nil {
		return nil, verr
	}

	resp := &EvaluateResponse{ArcLength: s.arcLengthOf(curve, tol)}

	if len(ts) > 0 {
		cvs, verr := s.curvaturesOf(curve, ts, "curvature_t")
		if verr != nil {
			return nil, verr
		}
		resp.Curvatures = cvs
	}
	if off != nil {
		or, verr := s.offsetOf(curve, off)
		if verr != nil {
			return nil, verr
		}
		resp.Offset = or
	}
	return resp, nil
}

// handleArcLength serves POST /api/v1/arc-length.
func (s *Server) handleArcLength(c *gin.Context) {
	var req ArcLengthRequest
	if verr := bind(c, &req); verr != nil {
		writeErr(c, verr)
		return
	}
	curve, verr := validate.ControlPoints(req.Points)
	if verr != nil {
		writeErr(c, verr)
		return
	}
	tol, verr := validate.Tolerance(req.Tolerance, s.cfg.DefaultTolerance)
	if verr != nil {
		writeErr(c, verr)
		return
	}
	c.JSON(http.StatusOK, s.arcLengthOf(curve, tol))
}

// handleCurvature serves POST /api/v1/curvature.
func (s *Server) handleCurvature(c *gin.Context) {
	var req CurvatureRequest
	if verr := bind(c, &req); verr != nil {
		writeErr(c, verr)
		return
	}
	ts := req.Ts
	if len(ts) == 0 {
		if req.T == nil {
			writeErr(c, validate.Invalid("t", "provide a parameter t or a non-empty list ts"))
			return
		}
		ts = []float64{*req.T}
	}
	curve, verr := validate.ControlPoints(req.Points)
	if verr != nil {
		writeErr(c, verr)
		return
	}
	cvs, verr := s.curvaturesOf(curve, ts, "ts")
	if verr != nil {
		writeErr(c, verr)
		return
	}
	c.JSON(http.StatusOK, CurvatureResponse{Curvatures: cvs})
}

// handleOffset serves POST /api/v1/offset.
func (s *Server) handleOffset(c *gin.Context) {
	var req OffsetRequest
	if verr := bind(c, &req); verr != nil {
		writeErr(c, verr)
		return
	}
	curve, verr := validate.ControlPoints(req.Points)
	if verr != nil {
		writeErr(c, verr)
		return
	}
	resp, verr := s.offsetOf(curve, &OffsetSpec{Distance: req.Distance, Segments: req.Segments})
	if verr != nil {
		writeErr(c, verr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleEvaluate serves POST /api/v1/evaluate.
func (s *Server) handleEvaluate(c *gin.Context) {
	var req EvaluateRequest
	if verr := bind(c, &req); verr != nil {
		writeErr(c, verr)
		return
	}
	resp, verr := s.computeAll(req.Points, req.Tolerance, req.CurvatureT, req.Offset)
	if verr != nil {
		writeErr(c, verr)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleBatch serves POST /api/v1/batch. Every item goes through the same
// computeAll pipeline as /evaluate; a failing item only fails itself.
func (s *Server) handleBatch(c *gin.Context) {
	var req BatchRequest
	if verr := bind(c, &req); verr != nil {
		writeErr(c, verr)
		return
	}
	if len(req.Curves) == 0 {
		writeErr(c, validate.Invalid("curves", "batch must contain at least one curve"))
		return
	}
	if len(req.Curves) > s.cfg.MaxBatchSize {
		writeErr(c, validate.Invalid("curves", "batch size %d exceeds the limit %d", len(req.Curves), s.cfg.MaxBatchSize))
		return
	}
	results := make([]BatchItemResult, len(req.Curves))
	for i, item := range req.Curves {
		id := item.ID
		if id == "" {
			id = strconv.Itoa(i)
		}
		res := BatchItemResult{ID: id}
		// Each item is independent: only its own request fields decide what
		// gets computed. Never carry parameters over from a previous item.
		resp, verr := s.computeAll(item.Points, item.Tolerance, item.CurvatureT, item.Offset)
		if verr != nil {
			res.Error = verr
		} else {
			res.OK = true
			res.ArcLength = resp.ArcLength
			res.Curvatures = resp.Curvatures
			res.Offset = resp.Offset
		}
		results[i] = res
	}
	c.JSON(http.StatusOK, BatchResponse{Results: results})
}

// handleConfig serves GET /api/v1/config: read-only echo of the service
// configuration plus a health status for monitoring.
func (s *Server) handleConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"uptime_seconds": time.Since(s.started).Seconds(),
		"config": gin.H{
			"version":                 s.cfg.Version,
			"default_tolerance":       s.cfg.DefaultTolerance,
			"min_refinement_depth":    s.cfg.MinRefinementDepth,
			"max_refinement_depth":    s.cfg.MaxRefinementDepth,
			"default_offset_segments": s.cfg.DefaultOffsetSegments,
			"max_offset_segments":     s.cfg.MaxOffsetSegments,
			"max_batch_size":          s.cfg.MaxBatchSize,
			"max_curvature_points":    s.cfg.MaxCurvaturePoints,
		},
	})
}

// handleDemo serves GET /api/v1/demo with the preloaded bow curve.
func (s *Server) handleDemo(c *gin.Context) {
	al := s.arcLengthOf(demoCurve, s.cfg.DefaultTolerance)
	chord := demoCurve.P[3].Sub(demoCurve.P[0]).Norm()
	cvs, _ := s.curvaturesOf(demoCurve, []float64{0.25, 0.5, 0.75}, "t")
	c.JSON(http.StatusOK, gin.H{
		"description": "preloaded bow-shaped cubic Bézier: arc length (exactly 2) is clearly larger than the chord (exactly 1)",
		"points": []PointOut{
			{X: demoCurve.P[0].X, Y: demoCurve.P[0].Y},
			{X: demoCurve.P[1].X, Y: demoCurve.P[1].Y},
			{X: demoCurve.P[2].X, Y: demoCurve.P[2].Y},
			{X: demoCurve.P[3].X, Y: demoCurve.P[3].Y},
		},
		"chord_length":          chord,
		"arc_length":            al,
		"length_to_chord_ratio": al.Length / chord,
		"curvatures":            cvs,
	})
}
