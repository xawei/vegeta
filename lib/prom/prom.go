package prom

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

//PrometheusMetrics vegeta metrics observer with exposition as Prometheus metrics endpoint
type PrometheusMetrics struct {
	requestSecondsHistogram *prometheus.HistogramVec
	requestBytesInCounter   *prometheus.CounterVec
	requestBytesOutCounter  *prometheus.CounterVec
	requestFailCounter      *prometheus.CounterVec
	srv                     http.Server
	registry                *prometheus.Registry
}

//NewPrometheusMetrics same as NewPrometheusMetricsWithParams with default params:
func NewPrometheusMetrics() (*PrometheusMetrics, error) {
	return NewPrometheusMetricsWithParams("0.0.0.0:8880")
}

// NewPrometheusMetricsWithParams start a new Prometheus observer instance for exposing
// metrics to Prometheus Servers.
// Only one PrometheusMetrics can be instantiated at a time because of the underlaying
// mechanisms of promauto.
// Some metrics are requests/s, bytes in/out/s and failures/s
// Options are:
//   - bindURL: "[host]:[port]/[path]" to bind the listening socket to
// For example, after using NewPrometheusMetricsWithParams("0.0.0.0:8880"),
// during an "attack" you can call "curl http://127.0.0.0:8880" to see current metrics.
// This endpoint can be configured in scrapper section of your Prometheus server.
func NewPrometheusMetricsWithParams(bindURL string) (*PrometheusMetrics, error) {

	//parse bind url elements
	re := regexp.MustCompile("(.+):([0-9]+)")
	rr := re.FindAllStringSubmatch(bindURL, 3)
	bindHost := ""
	bindPort := 0
	var err error
	if len(rr) == 1 {
		if len(rr[0]) == 3 {
			bindHost = rr[0][1]
			bindPort, err = strconv.Atoi(rr[0][2])
			if err != nil {
				return nil, err
			}
		}
	}
	if bindHost == "" {
		return nil, fmt.Errorf("Invalid bindURL %s. Must be in format '0.0.0.0:8880'", bindURL)
	}

	pm := &PrometheusMetrics{
		registry: prometheus.NewRegistry(),
	}

	pm.requestSecondsHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_seconds",
		Help:    "Request latency",
		Buckets: []float64{0.1, 0.2, 0.5, 1.0, 2.0, 5.0, 10.0, 20, 50},
	}, []string{
		"method",
		"url",
		"status",
	})
	pm.registry.MustRegister(pm.requestSecondsHistogram)

	pm.requestBytesInCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "request_bytes_in",
		Help: "Bytes received from servers as response to requests",
	}, []string{
		"method",
		"url",
		"status",
	})
	pm.registry.MustRegister(pm.requestBytesInCounter)

	pm.requestBytesOutCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "request_bytes_out",
		Help: "Bytes sent to servers during requests",
	}, []string{
		"method",
		"url",
		"status",
	})
	pm.registry.MustRegister(pm.requestBytesOutCounter)

	pm.requestFailCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "request_fail_count",
		Help: "Internal failures that prevented a hit to the target server",
	}, []string{
		"method",
		"url",
		"message",
	})
	pm.registry.MustRegister(pm.requestFailCounter)

	//setup prometheus metrics http server
	pm.srv = http.Server{
		Addr:    fmt.Sprintf("%s:%d", bindHost, bindPort),
		Handler: promhttp.HandlerFor(pm.registry, promhttp.HandlerOpts{}),
	}

	go func() {
		pm.srv.ListenAndServe()
	}()

	return pm, nil
}

//Close shutdown http server exposing Prometheus metrics and unregister
//all prometheus collectors
func (pm *PrometheusMetrics) Close() error {
	prometheus.Unregister(pm.requestSecondsHistogram)
	prometheus.Unregister(pm.requestBytesInCounter)
	prometheus.Unregister(pm.requestBytesOutCounter)
	prometheus.Unregister(pm.requestFailCounter)
	return pm.srv.Shutdown(context.Background())
}

//Observe register metrics about hit results
func (pm *PrometheusMetrics) Observe(res *vegeta.Result) {
	code := strconv.FormatUint(uint64(res.Code), 10)
	pm.requestBytesInCounter.WithLabelValues(res.Method, res.URL, code).Add(float64(res.BytesIn))
	pm.requestBytesOutCounter.WithLabelValues(res.Method, res.URL, code).Add(float64(res.BytesOut))
	pm.requestSecondsHistogram.WithLabelValues(res.Method, res.URL, code).Observe(float64(res.Latency) / float64(time.Second))
	if res.Error != "" {
		pm.requestFailCounter.WithLabelValues(res.Method, res.URL, res.Error)
	}
}
