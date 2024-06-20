package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/status"
)

var (
	enabled bool

	registry     *prometheus.Registry
	grpcReqs     *prometheus.CounterVec
	grpcDurs     *prometheus.HistogramVec
	sessionTotal *prometheus.CounterVec
	sessionCount *prometheus.GaugeVec
)

// Init 初始化metrics
func Init() *prometheus.Registry {
	if enabled {
		return registry
	}

	grpcReqs = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Number of grpc requests",
		},
		[]string{"method", "code"},
	)

	grpcDurs = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "Duration of grpc requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	sessionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "session_total",
			Help: "Total number of sessions",
		},
		[]string{"type"},
	)

	sessionCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "session_count",
			Help: "Number of sessions",
		},
		[]string{"type"},
	)

	registry = prometheus.NewRegistry()
	registry.MustRegister(grpcReqs)
	registry.MustRegister(grpcDurs)
	registry.MustRegister(sessionTotal)
	registry.MustRegister(sessionCount)

	enabled = true

	return registry
}

// IncrGRPCRequests 统计grpc请求
func IncrGRPCRequests(method string, err error, duration time.Duration) {
	if !enabled {
		return
	}

	grpcReqs.With(prometheus.Labels{
		"method": method,
		"code":   status.Code(err).String(),
	}).Inc()

	grpcDurs.With(prometheus.Labels{
		"method": method,
	}).Observe(duration.Seconds())
}

// IncrGatewaySession 增加网关session计数
func IncrGatewaySession(sessionType string) {
	if !enabled {
		return
	}

	sessionTotal.WithLabelValues(sessionType).Inc()
	sessionCount.WithLabelValues(sessionType).Inc()
}

// DecrGatewaySession 减少网关session计数
func DecrGatewaySession(sessionType string) {
	if !enabled {
		return
	}

	sessionCount.WithLabelValues(sessionType).Dec()
}
