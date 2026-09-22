package api

import "beziersvc/internal/validate"

// PointOut is a 2D point in a response body.
type PointOut struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ArcLengthRequest is the body of POST /api/v1/arc-length.
type ArcLengthRequest struct {
	Points    []validate.PointInput `json:"points"`
	Tolerance *float64              `json:"tolerance"`
}

// ArcLengthResponse reports a converged arc-length integration.
type ArcLengthResponse struct {
	Length    float64 `json:"length"`
	Segments  int     `json:"segments"`
	Residual  float64 `json:"residual"`
	Converged bool    `json:"converged"`
	Tolerance float64 `json:"tolerance"`
}

// CurvatureRequest is the body of POST /api/v1/curvature.
// Provide either a single t or a list ts.
type CurvatureRequest struct {
	Points []validate.PointInput `json:"points"`
	T      *float64              `json:"t"`
	Ts     []float64             `json:"ts"`
}

// CurvatureValue pairs a parameter with its curvature.
type CurvatureValue struct {
	T         float64 `json:"t"`
	Curvature float64 `json:"curvature"`
}

// CurvatureResponse is the body of the POST /api/v1/curvature response.
type CurvatureResponse struct {
	Curvatures []CurvatureValue `json:"curvatures"`
}

// OffsetSpec selects an offset computation: distance (required) and the
// polyline resolution (optional).
type OffsetSpec struct {
	Distance *float64 `json:"distance"`
	Segments *int     `json:"segments"`
}

// OffsetRequest is the body of POST /api/v1/offset.
type OffsetRequest struct {
	Points   []validate.PointInput `json:"points"`
	Distance *float64              `json:"distance"`
	Segments *int                  `json:"segments"`
}

// OffsetResponse is a normal-equidistant offset polyline.
type OffsetResponse struct {
	Distance float64    `json:"distance"`
	Segments int        `json:"segments"`
	Polyline []PointOut `json:"polyline"`
}

// EvaluateRequest is the body of POST /api/v1/evaluate: arc length always,
// curvature and offset on demand.
type EvaluateRequest struct {
	Points     []validate.PointInput `json:"points"`
	Tolerance  *float64              `json:"tolerance"`
	CurvatureT []float64             `json:"curvature_t"`
	Offset     *OffsetSpec           `json:"offset"`
}

// EvaluateResponse bundles the three curve quantities.
type EvaluateResponse struct {
	ArcLength  *ArcLengthResponse `json:"arc_length"`
	Curvatures []CurvatureValue   `json:"curvatures,omitempty"`
	Offset     *OffsetResponse    `json:"offset,omitempty"`
}

// BatchItem is one curve job inside POST /api/v1/batch.
type BatchItem struct {
	ID         string                `json:"id"`
	Points     []validate.PointInput `json:"points"`
	Tolerance  *float64              `json:"tolerance"`
	CurvatureT []float64             `json:"curvature_t"`
	Offset     *OffsetSpec           `json:"offset"`
}

// BatchRequest is the body of POST /api/v1/batch.
type BatchRequest struct {
	Curves []BatchItem `json:"curves"`
}

// BatchItemResult is the per-curve outcome: either the full evaluation or a
// typed error — one bad item never affects the others.
type BatchItemResult struct {
	ID         string             `json:"id"`
	OK         bool               `json:"ok"`
	ArcLength  *ArcLengthResponse `json:"arc_length,omitempty"`
	Curvatures []CurvatureValue   `json:"curvatures,omitempty"`
	Offset     *OffsetResponse    `json:"offset,omitempty"`
	Error      *validate.Error    `json:"error,omitempty"`
}

// BatchResponse is the body of the POST /api/v1/batch response.
type BatchResponse struct {
	Results []BatchItemResult `json:"results"`
}

// errorBody is the uniform error envelope: {"error": {...}}.
type errorBody struct {
	Error *validate.Error `json:"error"`
}
