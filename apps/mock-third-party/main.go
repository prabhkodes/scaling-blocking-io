// mock-third-party stands in for the slow external API. It blocks each
// request for a duration drawn from a lognormal distribution, which is a
// reasonable shape for real-world API latency: mostly clustered around a
// median with a long right tail (occasional very slow calls), never negative.
package main

import (
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

type config struct {
	medianMS float64
	sigma    float64
	minMS    float64
	maxMS    float64
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func loadConfig() config {
	return config{
		// Defaults reproduce the "8s+, long-tailed" latency observed in
		// production: median 8000ms, sigma tuned so p99 lands a few
		// multiples above the median rather than symmetric jitter.
		medianMS: envFloat("MEDIAN_MS", 8000),
		sigma:    envFloat("SIGMA", 0.5),
		minMS:    envFloat("MIN_MS", 200),
		maxMS:    envFloat("MAX_MS", 60000),
	}
}

// sampleLatency draws from a lognormal distribution parameterized by its
// median (not mean) so MEDIAN_MS is directly the value most requests cluster
// near, with sigma controlling tail heaviness.
func (c config) sampleLatency() time.Duration {
	mu := math.Log(c.medianMS)
	ms := math.Exp(mu + c.sigma*rand.NormFloat64())
	if ms < c.minMS {
		ms = c.minMS
	}
	if ms > c.maxMS {
		ms = c.maxMS
	}
	return time.Duration(ms * float64(time.Millisecond))
}

var inFlight int64

func main() {
	cfg := loadConfig()
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/delay", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&inFlight, 1)
		defer atomic.AddInt64(&inFlight, -1)

		var delay time.Duration
		if override := r.URL.Query().Get("ms"); override != "" {
			if v, err := strconv.Atoi(override); err == nil {
				delay = time.Duration(v) * time.Millisecond
			}
		}
		if delay == 0 {
			delay = cfg.sampleLatency()
		}

		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			// client gave up (e.g. load tester timeout) — stop holding the
			// connection open, mirrors a real upstream cancel
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"delayed_ms": delay.Milliseconds(),
		})
	})

	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"in_flight": atomic.LoadInt64(&inFlight),
			"config": map[string]float64{
				"median_ms": cfg.medianMS,
				"sigma":     cfg.sigma,
				"min_ms":    cfg.minMS,
				"max_ms":    cfg.maxMS,
			},
		})
	})

	log.Printf("mock-third-party listening on :%s (median=%vms sigma=%v min=%vms max=%vms)",
		port, cfg.medianMS, cfg.sigma, cfg.minMS, cfg.maxMS)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
