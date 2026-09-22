// Command server starts the Bézier curve kernel HTTP service.
package main

import (
	"log"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"beziersvc/internal/api"
)

func main() {
	cfg := api.DefaultConfig()

	if v := os.Getenv("BEZIER_DEFAULT_TOLERANCE"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 {
			log.Fatalf("invalid BEZIER_DEFAULT_TOLERANCE %q: must be a positive number", v)
		}
		cfg.DefaultTolerance = f
	}
	if v := os.Getenv("BEZIER_MAX_REFINEMENT_DEPTH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 40 {
			log.Fatalf("invalid BEZIER_MAX_REFINEMENT_DEPTH %q: must be in [1,40]", v)
		}
		cfg.MaxRefinementDepth = n
	}
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := api.NewServer(cfg)
	log.Printf("beziersvc %s listening on :%s (default tolerance %g, refinement depth [%d,%d])",
		cfg.Version, port, cfg.DefaultTolerance, cfg.MinRefinementDepth, cfg.MaxRefinementDepth)
	log.Printf("preloaded demo: GET /api/v1/demo (bow curve, arc length 2 vs chord 1)")
	if err := srv.Router().Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
