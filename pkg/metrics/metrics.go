package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTP 请求指标
var (
	// HTTP 请求总数
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chatim_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// HTTP 请求延迟
	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chatim_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "endpoint"},
	)

	// HTTP 请求大小
	HttpRequestSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chatim_http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "endpoint"},
	)

	// HTTP 响应大小
	HttpResponseSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chatim_http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "endpoint"},
	)
)

// 消息业务指标
var (
	// 消息发送总数
	MessagesSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chatim_messages_sent_total",
			Help: "Total number of messages sent",
		},
		[]string{"type", "status"}, // type: private/group, status: success/failed
	)

	// 消息发送延迟
	MessageSendDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chatim_message_send_duration_seconds",
			Help:    "Message send latency in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"type"},
	)

	// 消息拉取总数
	MessagesPulledTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chatim_messages_pulled_total",
			Help: "Total number of messages pulled",
		},
		[]string{"type"},
	)

	// 未读消息数
	UnreadMessagesCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chatim_unread_messages_count",
			Help: "Number of unread messages per user",
		},
		[]string{"user_id"},
	)
)

// Redis 指标
var (
	// Redis 操作总数
	RedisOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chatim_redis_operations_total",
			Help: "Total number of Redis operations",
		},
		[]string{"operation", "status"},
	)

	// Redis 操作延迟
	RedisOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chatim_redis_operation_duration_seconds",
			Help:    "Redis operation latency in seconds",
			Buckets: []float64{.0001, .0005, .001, .005, .01, .025, .05, .1},
		},
		[]string{"operation"},
	)

	// Redis Stream 积压消息数
	RedisStreamPendingMessages = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chatim_redis_stream_pending_messages",
			Help: "Number of pending messages in Redis Stream",
		},
		[]string{"stream_key"},
	)

	// Redis 连接池状态
	RedisPoolStats = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chatim_redis_pool_connections",
			Help: "Redis connection pool statistics",
		},
		[]string{"state"}, // state: hits, misses, timeouts, total, idle, stale
	)
)

// WebSocket 指标
var (
	// WebSocket 活跃连接数
	WebSocketActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chatim_websocket_active_connections",
			Help: "Number of active WebSocket connections",
		},
	)

	// WebSocket 消息推送总数
	WebSocketMessagesPushedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chatim_websocket_messages_pushed_total",
			Help: "Total number of messages pushed via WebSocket",
		},
		[]string{"type", "status"},
	)

	// WebSocket 连接持续时间
	WebSocketConnectionDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "chatim_websocket_connection_duration_seconds",
			Help:    "WebSocket connection duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600},
		},
	)
)

// 数据库指标
var (
	// 数据库查询总数
	DBQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chatim_db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "table", "status"},
	)

	// 数据库查询延迟
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chatim_db_query_duration_seconds",
			Help:    "Database query latency in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		},
		[]string{"operation", "table"},
	)

	// 数据库连接池状态
	DBConnectionPoolStats = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chatim_db_connection_pool",
			Help: "Database connection pool statistics",
		},
		[]string{"state"}, // state: open, in_use, idle, wait_count
	)
)

// gRPC 指标
var (
	// gRPC 请求总数
	GrpcRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chatim_grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"service", "method", "status"},
	)

	// gRPC 请求延迟
	GrpcRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chatim_grpc_request_duration_seconds",
			Help:    "gRPC request latency in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"service", "method"},
	)
)

// Go 运行时指标
var (
	// Goroutine 数量
	GoGoroutinesCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chatim_go_goroutines",
			Help: "Number of goroutines",
		},
	)

	// 内存分配
	GoMemoryAllocBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chatim_go_memory_alloc_bytes",
			Help: "Allocated memory in bytes",
		},
	)

	// 堆内存使用
	GoMemoryHeapBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chatim_go_memory_heap_bytes",
			Help: "Heap memory in bytes",
		},
	)

	// GC 暂停时间
	GoGCPauseDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "chatim_go_gc_pause_duration_seconds",
			Help:    "GC pause duration in seconds",
			Buckets: []float64{.00001, .00005, .0001, .0005, .001, .005, .01, .05},
		},
	)
)

// 业务指标
var (
	// 在线用户数
	OnlineUsersCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chatim_online_users",
			Help: "Number of online users",
		},
	)

	// 群组总数
	GroupsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chatim_groups_total",
			Help: "Total number of groups",
		},
	)

	// 好友关系总数
	FriendshipsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "chatim_friendships_total",
			Help: "Total number of friendships",
		},
	)
)
