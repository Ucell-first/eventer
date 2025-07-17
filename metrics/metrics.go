package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ProcessedLogs = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_processed_total",
			Help: "Total number of processed logs",
		},
		[]string{"status"},
	)

	BatchSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "batch_size",
			Help:    "Size of batches processed",
			Buckets: prometheus.LinearBuckets(50, 50, 20),
		},
		[]string{},
	)

	InsertDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "insert_duration_seconds",
			Help:    "Duration of batch inserts",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	KafkaLag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kafka_consumer_lag",
			Help: "Kafka consumer lag",
		},
		[]string{"topic", "partition"},
	)
)

func init() {
	prometheus.MustRegister(ProcessedLogs)
	prometheus.MustRegister(BatchSize)
	prometheus.MustRegister(InsertDuration)
	prometheus.MustRegister(KafkaLag)
}

func StartMetricsServer(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
}
