package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

var (
	totalRequests int
	concurrent    int
	url           string
)

func init() {
	flag.IntVar(&totalRequests, "n", 500, "Total number of requests")
	flag.IntVar(&concurrent, "c", 50, "Number of concurrent workers")
	flag.StringVar(&url, "url", "http://localhost:8081/api/v1/users", "Target URL")
}

func main() {
	flag.Parse()

	fmt.Printf("Load Test Configuration:\n")
	fmt.Printf("  Target URL: %s\n", url)
	fmt.Printf("  Total Requests: %d\n", totalRequests)
	fmt.Printf("  Concurrent Workers: %d\n\n", concurrent)

	var (
		successCount   int64
		errorCount     int64
		activeRequests int64 // 当前活跃的并发请求数
		wg             sync.WaitGroup
		sem            = make(chan struct{}, concurrent)
		latencies      = make([]time.Duration, 0, totalRequests)
		latencyMutex   sync.Mutex
	)

	startTime := time.Now()

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(reqNum int) {
			defer wg.Done()
			defer func() {
				<-sem // Release semaphore
				atomic.AddInt64(&activeRequests, -1)
			}()

			// 增加活跃请求计数
			active := atomic.AddInt64(&activeRequests, 1)

			// 每50个请求打印一次当前并发数
			if reqNum%50 == 0 {
				fmt.Printf("[Request %d] Active concurrent requests: %d\n", reqNum, active)
			}

			// Generate unique user data
			reqData := RegisterRequest{
				Username: fmt.Sprintf("testuser_%d_%d", time.Now().UnixNano(), reqNum),
				Password: "password123",
				Email:    fmt.Sprintf("test_%d_%d@example.com", time.Now().UnixNano(), reqNum),
			}

			jsonData, err := json.Marshal(reqData)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}

			reqStart := time.Now()
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
			latency := time.Since(reqStart)

			latencyMutex.Lock()
			latencies = append(latencies, latency)
			latencyMutex.Unlock()

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}
			defer resp.Body.Close()

			// Read and discard body
			io.Copy(io.Discard, resp.Body)

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&errorCount, 1)
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// Calculate statistics
	var totalLatency time.Duration
	minLatency := latencies[0]
	maxLatency := latencies[0]

	for _, lat := range latencies {
		totalLatency += lat
		if lat < minLatency {
			minLatency = lat
		}
		if lat > maxLatency {
			maxLatency = lat
		}
	}

	avgLatency := totalLatency / time.Duration(len(latencies))
	qps := float64(totalRequests) / totalDuration.Seconds()

	// Calculate theoretical QPS based on concurrency and latency
	theoreticalQPS := float64(concurrent) / (avgLatency.Seconds())

	// Print results
	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Duration: %.2fs\n", totalDuration.Seconds())
	fmt.Printf("Total Requests: %d\n", totalRequests)
	fmt.Printf("Success: %d\n", successCount)
	fmt.Printf("Failed: %d\n", errorCount)
	fmt.Printf("QPS: %.2f req/sec\n", qps)
	fmt.Printf("\nLatency:\n")
	fmt.Printf("  Min: %v\n", minLatency)
	fmt.Printf("  Avg: %v\n", avgLatency)
	fmt.Printf("  Max: %v\n", maxLatency)
	fmt.Printf("\nConcurrency Analysis:\n")
	fmt.Printf("  Configured Concurrency: %d\n", concurrent)
	fmt.Printf("  Theoretical QPS (Concurrent/AvgLatency): %.2f req/sec\n", theoreticalQPS)
	fmt.Printf("  Actual QPS: %.2f req/sec\n", qps)
	fmt.Printf("  Efficiency: %.1f%%\n", (qps/theoreticalQPS)*100)
}
