// Package api exposes the curve kernel over HTTP (the only entry point).
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"beziersvc/internal/arclength"
	"beziersvc/internal/validate"
)

// Config holds the service knobs echoed by GET /api/v1/config.
type Config struct {
	// DefaultTolerance is used when a request omits the arc-length tolerance.
	DefaultTolerance float64
	// MinRefinementDepth / MaxRefinementDepth bound the adaptive arc-length
	// refinement (the "加密上限").
	MinRefinementDepth int
	MaxRefinementDepth int
	// DefaultOffsetSegments / MaxOffsetSegments bound offset polylines.
	DefaultOffsetSegments int
	MaxOffsetSegments     int
	// MaxBatchSize bounds POST /api/v1/batch.
	MaxBatchSize int
	// MaxCurvaturePoints bounds the number of curvature parameters per curve.
	MaxCurvaturePoints int
	// Version is reported by the config endpoint.
	Version string
}

// DefaultConfig returns the out-of-the-box service configuration.
func DefaultConfig() Config {
	return Config{
		DefaultTolerance:      1e-9,
		MinRefinementDepth:    2,
		MaxRefinementDepth:    20,
		DefaultOffsetSegments: 64,
		MaxOffsetSegments:     4096,
		MaxBatchSize:          128,
		MaxCurvaturePoints:    256,
		Version:               "1.0.0",
	}
}

// Server is a stateless HTTP front-end over the curve kernel. All handlers
// are pure functions of their request, so concurrent requests never interfere.
type Server struct {
	cfg     Config
	started time.Time
}

// NewServer builds a Server with the given configuration.
func NewServer(cfg Config) *Server {
	return &Server{cfg: cfg, started: time.Now()}
}

func (s *Server) limits() arclength.Limits {
	return arclength.Limits{
		MinDepth: s.cfg.MinRefinementDepth,
		MaxDepth: s.cfg.MaxRefinementDepth,
	}
}

// Router wires all routes. HTTP is the only interface the kernel exposes.
func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(recovery(), gin.Logger())
	r.HandleMethodNotAllowed = true

	v1 := r.Group("/api/v1")
	v1.POST("/arc-length", s.handleArcLength)
	v1.POST("/curvature", s.handleCurvature)
	v1.POST("/offset", s.handleOffset)
	v1.POST("/evaluate", s.handleEvaluate)
	v1.POST("/batch", s.handleBatch)
	v1.GET("/config", s.handleConfig)
	v1.GET("/demo", s.handleDemo)

	r.NoRoute(func(c *gin.Context) {
		writeErr(c, &validate.Error{Type: validate.TypeNotFound, Message: "route not found"})
	})
	r.NoMethod(func(c *gin.Context) {
		writeErr(c, &validate.Error{Type: validate.TypeMethodNotAllowed, Message: "method not allowed on this route"})
	})
	return r
}

// recovery turns any unexpected panic into a typed 500 — the service never
// crashes or answers with an empty body.
func recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, errorBody{
					Error: &validate.Error{Type: validate.TypeInternal, Message: "internal error"},
				})
			}
		}()
		c.Next()
	}
}

// writeErr maps a typed error to its HTTP status and writes the envelope.
func writeErr(c *gin.Context, e *validate.Error) {
	status := http.StatusBadRequest
	switch e.Type {
	case validate.TypeSingular:
		status = http.StatusUnprocessableEntity
	case validate.TypeNotFound:
		status = http.StatusNotFound
	case validate.TypeMethodNotAllowed:
		status = http.StatusMethodNotAllowed
	case validate.TypeInternal:
		status = http.StatusInternalServerError
	}
	c.JSON(status, errorBody{Error: e})
}
