package profiling

import (
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"time"

	"ChatIM/pkg/logger"
	"ChatIM/pkg/metrics"

	"go.uber.org/zap"
)

// InitProfiling 初始化性能分析
// port: pprof HTTP 服务端口（如 6060）
func InitProfiling(port string) {
	// 启用 pprof HTTP 服务
	go func() {
		addr := "0.0.0.0:" + port // 改为 0.0.0.0 以便外部访问
		logger.Info("🔍 pprof server started", zap.String("addr", "http://localhost:"+port+"/debug/pprof/"))
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.Error("❌ Failed to start pprof server", zap.Error(err))
		}
	}()

	// 启用锁竞争检测
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	// 定期采集 Go 运行时指标
	go collectRuntimeMetrics()
}

// collectRuntimeMetrics 定期采集运行时指标并上报到 Prometheus
func collectRuntimeMetrics() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Goroutine 数量
		metrics.GoGoroutinesCount.Set(float64(runtime.NumGoroutine()))

		// 内存分配
		metrics.GoMemoryAllocBytes.Set(float64(m.Alloc))

		// 堆内存
		metrics.GoMemoryHeapBytes.Set(float64(m.HeapAlloc))

		// GC 暂停时间
		if m.NumGC > 0 {
			pauseNs := m.PauseNs[(m.NumGC+255)%256]
			metrics.GoGCPauseDuration.Observe(float64(pauseNs) / 1e9)
		}
	}
}
