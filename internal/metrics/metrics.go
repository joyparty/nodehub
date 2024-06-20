package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/status"
)

var (
	enabled bool

	registry *prometheus.Registry
	grpcReqs *prometheus.CounterVec
	grpcDurs *prometheus.HistogramVec
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

	registry = prometheus.NewRegistry()
	registry.MustRegister(grpcReqs)
	registry.MustRegister(grpcDurs)

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
