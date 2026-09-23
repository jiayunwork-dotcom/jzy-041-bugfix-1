package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"beziersvc/internal/api"
	"beziersvc/internal/bernstein"
	"beziersvc/internal/validate"
)

var (
	testSrv *httptest.Server
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	testSrv = httptest.NewServer(api.NewServer(api.DefaultConfig()).Router())
	defer testSrv.Close()
	m.Run()
}

// ---- helpers ---------------------------------------------------------------

func pts(c bernstein.Curve) []map[string]float64 {
	out := make([]map[string]float64, 4)
	for i, p := range c.P {
		out[i] = map[string]float64{"x": p.X, "y": p.Y}
	}
	return out
}

func sampleCurve() bernstein.Curve {
	return bernstein.Curve{P: [4]bernstein.Point{
		{X: 0.3, Y: -1.2}, {X: 2.5, Y: 0.4}, {X: -1.1, Y: 3.3}, {X: 4.0, Y: 0.7},
	}}
}

func bowCurve() bernstein.Curve {
	return bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1, Y: 2}, {X: 2, Y: 2}, {X: 3, Y: 0},
	}}
}

func lineCurve() bernstein.Curve {
	return bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0},
	}}
}

func cuspCurve() bernstein.Curve {
	return bernstein.Curve{P: [4]bernstein.Point{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 0}, {X: 1, Y: 0},
	}}
}

func post(t *testing.T, path string, body any) (int, []byte) {
	t.Helper()
	var rdr io.Reader
	if s, ok := body.(string); ok {
		rdr = strings.NewReader(s)
	} else {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	resp, err := http.Post(testSrv.URL+path, "application/json", rdr)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func get(t *testing.T, path string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(testSrv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func decode[T any](t *testing.T, data []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	return v
}

type errEnvelope struct {
	Error validate.Error `json:"error"`
}

func evaluate(t *testing.T, body map[string]any) api.EvaluateResponse {
	t.Helper()
	status, data := post(t, "/api/v1/evaluate", body)
	if status != http.StatusOK {
		t.Fatalf("evaluate status %d: %s", status, data)
	}
	return decode[api.EvaluateResponse](t, data)
}

// ---- geometric invariants over HTTP ---------------------------------------

func TestTranslationInvariance(t *testing.T) {
	c := sampleCurve()
	shift := bernstein.Point{X: 123.5, Y: -45.25}
	var moved bernstein.Curve
	for i, p := range c.P {
		moved.P[i] = p.Add(shift)
	}
	ts := []float64{0.1, 0.35, 0.6, 0.9}
	r0 := evaluate(t, map[string]any{"points": pts(c), "curvature_t": ts})
	r1 := evaluate(t, map[string]any{"points": pts(moved), "curvature_t": ts})
	if d := math.Abs(r0.ArcLength.Length - r1.ArcLength.Length); d > 1e-9*r0.ArcLength.Length {
		t.Fatalf("arc length changed under translation: %v vs %v", r0.ArcLength.Length, r1.ArcLength.Length)
	}
	for i := range ts {
		d := math.Abs(r0.Curvatures[i].Curvature - r1.Curvatures[i].Curvature)
		if d > 1e-9*math.Max(1, r0.Curvatures[i].Curvature) {
			t.Fatalf("curvature at t=%v changed under translation: %v vs %v",
				ts[i], r0.Curvatures[i].Curvature, r1.Curvatures[i].Curvature)
		}
	}
}

func TestScalingInvariance(t *testing.T) {
	c := sampleCurve()
	const k = 2.5
	var scaled bernstein.Curve
	for i, p := range c.P {
		scaled.P[i] = p.Scale(k)
	}
	ts := []float64{0.2, 0.5, 0.8}
	r0 := evaluate(t, map[string]any{"points": pts(c), "curvature_t": ts})
	r1 := evaluate(t, map[string]any{"points": pts(scaled), "curvature_t": ts})
	if want := k * r0.ArcLength.Length; math.Abs(r1.ArcLength.Length-want) > 1e-9*want {
		t.Fatalf("scaled arc length = %v, want %v", r1.ArcLength.Length, want)
	}
	for i := range ts {
		want := r0.Curvatures[i].Curvature / k
		if math.Abs(r1.Curvatures[i].Curvature-want) > 1e-9*math.Max(1, want) {
			t.Fatalf("scaled curvature at t=%v = %v, want %v", ts[i], r1.Curvatures[i].Curvature, want)
		}
	}
}

func TestOffsetZeroDistanceCoincides(t *testing.T) {
	c := sampleCurve()
	const seg = 24
	status, data := post(t, "/api/v1/offset", map[string]any{
		"points": pts(c), "distance": 0, "segments": seg,
	})
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, data)
	}
	resp := decode[api.OffsetResponse](t, data)
	if len(resp.Polyline) != seg+1 {
		t.Fatalf("polyline has %d points, want %d", len(resp.Polyline), seg+1)
	}
	for i, p := range resp.Polyline {
		want := c.Eval(float64(i) / seg)
		if d := math.Hypot(p.X-want.X, p.Y-want.Y); d > 1e-9 {
			t.Fatalf("offset point %d = %v, want %v for d=0", i, p, want)
		}
	}
}

func TestCollinearZeroCurvatureAndParallelOffset(t *testing.T) {
	c := lineCurve()
	// Curvature must be exactly zero everywhere on a straight line.
	status, data := post(t, "/api/v1/curvature", map[string]any{
		"points": pts(c), "ts": []float64{0, 0.25, 0.5, 0.75, 1},
	})
	if status != http.StatusOK {
		t.Fatalf("curvature status %d: %s", status, data)
	}
	cr := decode[api.CurvatureResponse](t, data)
	for _, cv := range cr.Curvatures {
		if cv.Curvature != 0 {
			t.Fatalf("curvature at t=%v = %v on a line, want 0", cv.T, cv.Curvature)
		}
	}
	// Offset must be a parallel straight line at distance exactly |d|.
	const d = 1.75
	status, data = post(t, "/api/v1/offset", map[string]any{
		"points": pts(c), "distance": d, "segments": 10,
	})
	if status != http.StatusOK {
		t.Fatalf("offset status %d: %s", status, data)
	}
	or := decode[api.OffsetResponse](t, data)
	for i, p := range or.Polyline {
		if math.Abs(math.Abs(p.Y)-d) > 1e-9 {
			t.Fatalf("offset point %d distance to line = %v, want %v", i, math.Abs(p.Y), d)
		}
		want := c.Eval(float64(i) / 10)
		if math.Abs(p.X-want.X) > 1e-9 {
			t.Fatalf("offset point %d x = %v, want %v (parallel line)", i, p.X, want.X)
		}
	}
}

func TestArcLengthRefinementAndChordBound(t *testing.T) {
	c := bowCurve()
	chord := 3.0
	polygon := 2*math.Sqrt(5) + 1 // |(1,2)| + |(1,0)| + |(1,-2)| = √5 + 1 + √5

	var prev *api.ArcLengthResponse
	for _, tol := range []float64{1e-1, 1e-4, 1e-8, 1e-12} {
		status, data := post(t, "/api/v1/arc-length", map[string]any{
			"points": pts(c), "tolerance": tol,
		})
		if status != http.StatusOK {
			t.Fatalf("status %d: %s", status, data)
		}
		cur := decode[api.ArcLengthResponse](t, data)
		if !cur.Converged {
			t.Fatalf("tol=%v did not converge", tol)
		}
		if cur.Residual > tol {
			t.Fatalf("tol=%v residual %v exceeds tolerance", tol, cur.Residual)
		}
		if cur.Length <= chord || cur.Length >= polygon {
			t.Fatalf("tol=%v length %v outside (chord %v, polygon %v)", tol, cur.Length, chord, polygon)
		}
		if prev != nil {
			if cur.Segments < prev.Segments {
				t.Fatalf("segments decreased: %d -> %d", prev.Segments, cur.Segments)
			}
			// Tighter tolerance must not move the value by more than the
			// looser tolerance — the sequence is genuinely converging.
			if d := math.Abs(cur.Length - prev.Length); d > prev.Tolerance {
				t.Fatalf("length jump %v exceeds previous tolerance %v", d, prev.Tolerance)
			}
		}
		c := cur
		prev = &c
	}
}

func TestDemoEndpoint(t *testing.T) {
	status, data := get(t, "/api/v1/demo")
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, data)
	}
	var resp struct {
		ChordLength float64               `json:"chord_length"`
		ArcLength   api.ArcLengthResponse `json:"arc_length"`
		Ratio       float64               `json:"length_to_chord_ratio"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if math.Abs(resp.ChordLength-1) > 1e-12 {
		t.Fatalf("demo chord = %v, want 1", resp.ChordLength)
	}
	// The preloaded bow has arc length exactly 2.
	if math.Abs(resp.ArcLength.Length-2) > 1e-9 {
		t.Fatalf("demo arc length = %v, want 2", resp.ArcLength.Length)
	}
	if resp.Ratio < 1.5 {
		t.Fatalf("demo length/chord ratio %v should be clearly above 1", resp.Ratio)
	}
}

// ---- input validation ------------------------------------------------------

func TestInvalidInputsRejected(t *testing.T) {
	good := pts(sampleCurve())
	cases := []struct {
		name string
		path string
		body any
	}{
		{"three points", "/api/v1/arc-length", map[string]any{"points": good[:3]}},
		{"five points", "/api/v1/arc-length", map[string]any{"points": append(good, good[0])}},
		{"missing coordinate", "/api/v1/arc-length", map[string]any{
			"points": []map[string]float64{{"x": 0, "y": 0}, {"x": 1}, {"x": 2, "y": 0}, {"x": 3, "y": 0}}}},
		{"non-numeric coordinate", "/api/v1/arc-length",
			`{"points":[{"x":0,"y":0},{"x":"abc","y":1},{"x":2,"y":0},{"x":3,"y":0}]}`},
		{"zero tolerance", "/api/v1/arc-length", map[string]any{"points": good, "tolerance": 0}},
		{"negative tolerance", "/api/v1/arc-length", map[string]any{"points": good, "tolerance": -1e-6}},
		{"t out of range", "/api/v1/curvature", map[string]any{"points": good, "t": 1.5}},
		{"missing t", "/api/v1/curvature", map[string]any{"points": good}},
		{"malformed json", "/api/v1/evaluate", `{"points": [`},
		{"empty body", "/api/v1/evaluate", ``},
		{"missing distance", "/api/v1/offset", map[string]any{"points": good}},
		{"zero segments", "/api/v1/offset", map[string]any{"points": good, "distance": 1, "segments": 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, data := post(t, tc.path, tc.body)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", status, data)
			}
			env := decode[errEnvelope](t, data)
			if env.Error.Type != validate.TypeInvalidInput {
				t.Fatalf("error type = %q, want invalid_input", env.Error.Type)
			}
			if env.Error.Message == "" {
				t.Fatal("error message must not be empty")
			}
		})
	}
}

func TestCuspCurvatureRejectedNotNaN(t *testing.T) {
	cusp := pts(cuspCurve())
	for _, tc := range []struct {
		name string
		path string
		body any
	}{
		{"curvature endpoint", "/api/v1/curvature", map[string]any{"points": cusp, "t": 0.5}},
		{"evaluate endpoint", "/api/v1/evaluate", map[string]any{"points": cusp, "curvature_t": []float64{0.5}}},
		{"offset endpoint", "/api/v1/offset", map[string]any{"points": cusp, "distance": 0.1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, data := post(t, tc.path, tc.body)
			if status != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422: %s", status, data)
			}
			env := decode[errEnvelope](t, data)
			if env.Error.Type != validate.TypeSingular {
				t.Fatalf("error type = %q, want singular", env.Error.Type)
			}
			if bytes.Contains(data, []byte("NaN")) || bytes.Contains(data, []byte("Inf")) {
				t.Fatalf("response leaks NaN/Inf: %s", data)
			}
		})
	}
	// Arc length of the same cusp curve is perfectly well-defined.
	status, data := post(t, "/api/v1/arc-length", map[string]any{"points": cusp})
	if status != http.StatusOK {
		t.Fatalf("cusp arc length should succeed, got %d: %s", status, data)
	}
}

// ---- batch -----------------------------------------------------------------

func TestBatchPartialFailure(t *testing.T) {
	body := map[string]any{"curves": []map[string]any{
		{"id": "bow", "points": pts(bowCurve()), "curvature_t": []float64{0.5}},
		{"id": "bad", "points": pts(bowCurve())[:3]},
		{"id": "cusp", "points": pts(cuspCurve()), "curvature_t": []float64{0.5}},
		{"id": "line", "points": pts(lineCurve()), "offset": map[string]any{"distance": 0.5}},
	}}
	status, data := post(t, "/api/v1/batch", body)
	if status != http.StatusOK {
		t.Fatalf("batch status = %d, want 200: %s", status, data)
	}
	resp := decode[api.BatchResponse](t, data)
	if len(resp.Results) != 4 {
		t.Fatalf("got %d results, want 4", len(resp.Results))
	}

	if !resp.Results[0].OK || resp.Results[0].ArcLength == nil || resp.Results[0].Curvatures == nil {
		t.Fatalf("bow item should succeed: %+v", resp.Results[0])
	}
	if resp.Results[1].OK || resp.Results[1].Error == nil ||
		resp.Results[1].Error.Type != validate.TypeInvalidInput {
		t.Fatalf("bad item should fail with invalid_input: %+v", resp.Results[1])
	}
	if resp.Results[2].OK || resp.Results[2].Error == nil ||
		resp.Results[2].Error.Type != validate.TypeSingular {
		t.Fatalf("cusp item should fail with singular: %+v", resp.Results[2])
	}
	if !resp.Results[3].OK || resp.Results[3].Offset == nil ||
		len(resp.Results[3].Offset.Polyline) != 65 { // default 64 segments + 1
		t.Fatalf("line item should succeed with default 64-segment offset: %+v", resp.Results[3])
	}
}

func TestBatchMatchesSingleEvaluate(t *testing.T) {
	c := sampleCurve()
	ts := []float64{0.15, 0.55, 0.95}
	single := evaluate(t, map[string]any{
		"points": pts(c), "tolerance": 1e-10, "curvature_t": ts,
		"offset": map[string]any{"distance": 0.3, "segments": 12},
	})
	status, data := post(t, "/api/v1/batch", map[string]any{"curves": []map[string]any{
		{"id": "x", "points": pts(c), "tolerance": 1e-10, "curvature_t": ts,
			"offset": map[string]any{"distance": 0.3, "segments": 12}},
	}})
	if status != http.StatusOK {
		t.Fatalf("batch status = %d: %s", status, data)
	}
	resp := decode[api.BatchResponse](t, data)
	got := resp.Results[0]
	if !got.OK {
		t.Fatalf("batch item failed: %+v", got.Error)
	}
	// Same pipeline, same numbers — bit for bit.
	if got.ArcLength.Length != single.ArcLength.Length ||
		got.ArcLength.Segments != single.ArcLength.Segments ||
		got.ArcLength.Residual != single.ArcLength.Residual {
		t.Fatalf("batch arc length %+v != single %+v", got.ArcLength, single.ArcLength)
	}
	for i := range ts {
		if got.Curvatures[i].Curvature != single.Curvatures[i].Curvature {
			t.Fatalf("curvature mismatch at t=%v: %v vs %v",
				ts[i], got.Curvatures[i].Curvature, single.Curvatures[i].Curvature)
		}
	}
	for i, p := range got.Offset.Polyline {
		q := single.Offset.Polyline[i]
		if p != q {
			t.Fatalf("offset point %d differs: %v vs %v", i, p, q)
		}
	}
}

func TestBatchEnvelopeValidation(t *testing.T) {
	status, data := post(t, "/api/v1/batch", map[string]any{"curves": []any{}})
	if status != http.StatusBadRequest {
		t.Fatalf("empty batch: status %d: %s", status, data)
	}
	big := make([]map[string]any, 200)
	for i := range big {
		big[i] = map[string]any{"points": pts(lineCurve())}
	}
	status, _ = post(t, "/api/v1/batch", map[string]any{"curves": big})
	if status != http.StatusBadRequest {
		t.Fatalf("oversized batch: status %d, want 400", status)
	}
}

// TestBatchNoParameterLeak guards item independence: a curve without
// curvature_t (or offset) must never inherit a previous item's parameters,
// regardless of order or how many bare items sit between parameter-bearing
// ones.
func TestBatchNoParameterLeak(t *testing.T) {
	withParams := map[string]any{
		"points": pts(bowCurve()), "curvature_t": []float64{0.25, 0.75},
		"offset": map[string]any{"distance": 0.3, "segments": 8},
	}
	bare := func(c bernstein.Curve) map[string]any {
		return map[string]any{"points": pts(c)}
	}

	status, data := post(t, "/api/v1/batch", map[string]any{"curves": []map[string]any{
		withParams,
		bare(sampleCurve()),
		bare(lineCurve()),
		withParams,
	}})
	if status != http.StatusOK {
		t.Fatalf("batch status = %d: %s", status, data)
	}
	resp := decode[api.BatchResponse](t, data)
	if len(resp.Results) != 4 {
		t.Fatalf("got %d results, want 4", len(resp.Results))
	}
	// Items 0 and 3 carried the parameters; 1 and 2 must stay bare even
	// though a parameter-bearing item precedes them.
	for i, wantParams := range []bool{true, false, false, true} {
		r := resp.Results[i]
		if !r.OK {
			t.Fatalf("item %d should succeed: %+v", i, r.Error)
		}
		if r.ArcLength == nil {
			t.Fatalf("item %d always gets arc length", i)
		}
		if wantParams {
			if len(r.Curvatures) != 2 || r.Offset == nil {
				t.Fatalf("item %d should keep its own curvature/offset: %+v", i, r)
			}
		} else if r.Curvatures != nil || r.Offset != nil {
			t.Fatalf("item %d leaked parameters from an earlier item: %+v", i, r)
		}
	}

	// Reversed order: the bare item first must stay bare, and the item
	// carrying parameters still gets exactly what it asked for.
	status, data = post(t, "/api/v1/batch", map[string]any{"curves": []map[string]any{
		bare(sampleCurve()),
		withParams,
	}})
	if status != http.StatusOK {
		t.Fatalf("reversed batch status = %d: %s", status, data)
	}
	rev := decode[api.BatchResponse](t, data)
	if rev.Results[0].Curvatures != nil || rev.Results[0].Offset != nil {
		t.Fatalf("first bare item should have no curvature/offset: %+v", rev.Results[0])
	}
	if len(rev.Results[1].Curvatures) != 2 || rev.Results[1].Offset == nil {
		t.Fatalf("second item should keep its own parameters: %+v", rev.Results[1])
	}
}

// ---- config / status -------------------------------------------------------

func TestConfigEndpoint(t *testing.T) {
	status, data := get(t, "/api/v1/config")
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, data)
	}
	var resp struct {
		Status        string  `json:"status"`
		UptimeSeconds float64 `json:"uptime_seconds"`
		Config        struct {
			DefaultTolerance   float64 `json:"default_tolerance"`
			MaxRefinementDepth int     `json:"max_refinement_depth"`
			MaxBatchSize       int     `json:"max_batch_size"`
		} `json:"config"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok", resp.Status)
	}
	if resp.UptimeSeconds < 0 {
		t.Fatalf("uptime = %v", resp.UptimeSeconds)
	}
	if resp.Config.DefaultTolerance != 1e-9 || resp.Config.MaxRefinementDepth != 20 || resp.Config.MaxBatchSize != 128 {
		t.Fatalf("unexpected config echo: %+v", resp.Config)
	}
}

func TestUnknownRouteAndMethod(t *testing.T) {
	status, data := get(t, "/api/v1/nope")
	if status != http.StatusNotFound {
		t.Fatalf("status %d, want 404", status)
	}
	env := decode[errEnvelope](t, data)
	if env.Error.Type != validate.TypeNotFound {
		t.Fatalf("type = %q, want not_found", env.Error.Type)
	}
	req, _ := http.NewRequest(http.MethodGet, testSrv.URL+"/api/v1/arc-length", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET on POST route: status %d, want 405", resp.StatusCode)
	}
}

// ---- concurrency -----------------------------------------------------------

func TestConcurrentRequestsNoCrossTalk(t *testing.T) {
	base := sampleCurve()
	ts := []float64{0.3, 0.7}

	// Expected values per scale factor, computed once sequentially.
	ks := []float64{0.5, 1, 1.5, 2, 3, 5, 8, 13}
	type want struct {
		length float64
		kappas []float64
	}
	wants := make([]want, len(ks))
	for i, k := range ks {
		var scaled bernstein.Curve
		for j, p := range base.P {
			scaled.P[j] = p.Scale(k)
		}
		r := evaluate(t, map[string]any{"points": pts(scaled), "curvature_t": ts})
		w := want{length: r.ArcLength.Length}
		for _, cv := range r.Curvatures {
			w.kappas = append(w.kappas, cv.Curvature)
		}
		wants[i] = w
	}

	var wg sync.WaitGroup
	errs := make(chan string, 1024)
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for iter := 0; iter < 25; iter++ {
				i := (worker + iter) % len(ks)
				k := ks[i]
				var scaled bernstein.Curve
				for j, p := range base.P {
					scaled.P[j] = p.Scale(k)
				}
				body, _ := json.Marshal(map[string]any{"points": pts(scaled), "curvature_t": ts})
				resp, err := http.Post(testSrv.URL+"/api/v1/evaluate", "application/json", bytes.NewReader(body))
				if err != nil {
					errs <- fmt.Sprintf("worker %d: %v", worker, err)
					return
				}
				data, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					errs <- fmt.Sprintf("worker %d: status %d: %s", worker, resp.StatusCode, data)
					return
				}
				var got api.EvaluateResponse
				if err := json.Unmarshal(data, &got); err != nil {
					errs <- fmt.Sprintf("worker %d: decode: %v", worker, err)
					return
				}
				if got.ArcLength.Length != wants[i].length {
					errs <- fmt.Sprintf("worker %d: length %v != expected %v (cross-talk?)",
						worker, got.ArcLength.Length, wants[i].length)
					return
				}
				for j := range ts {
					if got.Curvatures[j].Curvature != wants[i].kappas[j] {
						errs <- fmt.Sprintf("worker %d: curvature mismatch", worker)
						return
					}
				}
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}
