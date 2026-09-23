// loadtester is an open-loop load generator: it issues requests on a fixed
// schedule (one every 1/rate seconds) regardless of whether prior requests
// have finished, rather than waiting for a response before sending the next
// one (a "closed-loop" design). Closed-loop generators silently understate
// tail latency once the server saturates — once a request stalls, the
// generator just stops sending, so the very degradation you're trying to
// measure suppresses the measurement. Open-loop keeps offering load at the
// intended rate and records what actually happens, including outright
// failures to even get a response — which is the point, here: distinguishing
// "the app returned an error" from "the connection never got a response at
// all" is exactly the mystery this whole project is chasing.
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type result struct {
	sentAt  time.Time
	latency time.Duration
	status  int    // 0 if the request never got an HTTP response at all
	errMsg  string // non-empty for transport-level failures (refused/reset/timeout)
}

func main() {
	targetURL := flag.String("url", "", "target URL (required)")
	rate := flag.Float64("rate", 10, "requests per second, open-loop fixed schedule")
	duration := flag.Duration("duration", 30*time.Second, "test duration")
	timeout := flag.Duration("timeout", 65*time.Second, "per-request timeout")
	outPath := flag.String("out", "", "optional CSV path for per-request results")
	maxInFlight := flag.Int("max-in-flight", 20000, "safety cap on concurrent outstanding requests (protects the load generator itself, not the target)")
	flag.Parse()

	if *targetURL == "" {
		fmt.Fprintln(os.Stderr, "usage: loadtester -url http://host:port/path -rate 200 -duration 30s")
		os.Exit(1)
	}

	interval := time.Duration(float64(time.Second) / *rate)
	// Go's default Transport only keeps 2 idle connections per host, so at
	// real concurrency (hundreds+ in flight against one target) most
	// connections would be closed and re-handshaked instead of reused,
	// making the *generator* the bottleneck rather than the app under test.
	transport := &http.Transport{
		MaxIdleConns:        0, // unlimited
		MaxIdleConnsPerHost: *maxInFlight,
		MaxConnsPerHost:     0, // unlimited concurrent — we cap concurrency ourselves via sem
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{Timeout: *timeout, Transport: transport}

	sem := make(chan struct{}, *maxInFlight)
	var sent, capHits int64

	resultsCh := make(chan result, 4096)
	var wg sync.WaitGroup

	fmt.Printf("target=%s rate=%.1f/s duration=%s timeout=%s\n", *targetURL, *rate, *duration, *timeout)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		deadline := time.Now().Add(*duration)

		for now := range ticker.C {
			if now.After(deadline) {
				break
			}
			select {
			case sem <- struct{}{}:
			default:
				atomic.AddInt64(&capHits, 1)
				continue
			}
			atomic.AddInt64(&sent, 1)
			wg.Add(1)
			go func(sentAt time.Time) {
				defer wg.Done()
				defer func() { <-sem }()
				resultsCh <- doRequest(client, *targetURL, sentAt)
			}(now)
		}
		wg.Wait()
		close(resultsCh)
	}()

	var results []result
	for r := range resultsCh {
		results = append(results, r)
	}

	printSummary(results, atomic.LoadInt64(&sent), atomic.LoadInt64(&capHits), *duration, *rate)

	if *outPath != "" {
		if err := writeCSV(*outPath, results); err != nil {
			fmt.Fprintf(os.Stderr, "failed writing CSV: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\nper-request results written to %s\n", *outPath)
	}
}

func doRequest(client *http.Client, url string, sentAt time.Time) result {
	// client.Timeout already bounds the whole round trip (and cancels the
	// request's context when it fires), so no separate context timeout needed.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return result{sentAt: sentAt, errMsg: err.Error()}
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return result{sentAt: sentAt, latency: latency, errMsg: err.Error()}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return result{sentAt: sentAt, latency: latency, status: resp.StatusCode}
}

func printSummary(results []result, sent, capHits int64, duration time.Duration, targetRate float64) {
	var success, appError, unreachable int
	var successLatencies []time.Duration
	var failLatencySum time.Duration

	for _, r := range results {
		switch {
		case r.errMsg != "":
			unreachable++
			failLatencySum += r.latency
		case r.status >= 200 && r.status < 300:
			success++
			successLatencies = append(successLatencies, r.latency)
		default:
			appError++
			failLatencySum += r.latency
		}
	}

	completed := len(results)
	achievedRate := float64(sent) / duration.Seconds()

	fmt.Println()
	fmt.Println("=== summary ===")
	fmt.Printf("requested rate:   %.1f req/s\n", targetRate)
	fmt.Printf("achieved rate:    %.1f req/s (%d sent)\n", achievedRate, sent)
	if capHits > 0 {
		fmt.Printf("WARNING: %d scheduled requests were dropped by the generator's own in-flight cap — achieved rate fell short of target; target overwhelmed the load generator, not just the server under test\n", capHits)
	}
	fmt.Printf("completed:        %d\n", completed)
	fmt.Printf("  success (2xx):        %d (%.2f%%)\n", success, pct(success, completed))
	fmt.Printf("  app error (non-2xx):  %d (%.2f%%)\n", appError, pct(appError, completed))
	fmt.Printf("  unreachable/no-resp:  %d (%.2f%%)\n", unreachable, pct(unreachable, completed))

	if len(successLatencies) > 0 {
		sort.Slice(successLatencies, func(i, j int) bool { return successLatencies[i] < successLatencies[j] })
		fmt.Println("latency (successful requests only):")
		fmt.Printf("  p50=%s p90=%s p95=%s p99=%s max=%s\n",
			percentile(successLatencies, 0.50),
			percentile(successLatencies, 0.90),
			percentile(successLatencies, 0.95),
			percentile(successLatencies, 0.99),
			successLatencies[len(successLatencies)-1],
		)
	}
	if appError+unreachable > 0 {
		avgFail := failLatencySum / time.Duration(appError+unreachable)
		fmt.Printf("avg time-to-failure (errors + unreachable): %s\n", avgFail)
	}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}

func writeCSV(path string, results []result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"sent_at_unix_ms", "latency_ms", "status", "error"}); err != nil {
		return err
	}
	for _, r := range results {
		row := []string{
			strconv.FormatInt(r.sentAt.UnixMilli(), 10),
			strconv.FormatInt(r.latency.Milliseconds(), 10),
			strconv.Itoa(r.status),
			r.errMsg,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
