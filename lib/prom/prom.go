package prom

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
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
	listenPort              net.Listener
}

//NewPrometheusMetrics same as NewPrometheusMetricsWithParams with default params:
//bindHost=0.0.0.0, bindPort=8880 and metricsPath=/metrics
func NewPrometheusMetrics() (PrometheusMetrics, error) {
	return NewPrometheusMetricsWithParams("0.0.0.0", 8880, "/metrics")
}

// NewPrometheusMetricsWithParams start a new Prometheus observer instance for exposing
// metrics to Prometheus Servers.
// Only one PrometheusMetrics can be instantiated at a time because of the underlaying
// mechanisms of promauto.
// Some metrics are requests/s, bytes in/out/s and failures/s
// Options are:
//   - bindHost: host to bind the listening socket to
//   - bindPort: port to bind the listening socket to
//   - metricsPath: http path that will be used to get metrics
// For example, after using NewPrometheusMetricsWithParams("0.0.0.0", 8880, "/metrics"),
// during an "attack" you can call "curl http://127.0.0.0:8880/metrics" to see current metrics.
// This endpoint can be configured in scrapper section of your Prometheus server.
func NewPrometheusMetricsWithParams(bindHost string, bindPort int, metricsPath string) (PrometheusMetrics, error) {

	pm := PrometheusMetrics{}

	pm.requestSecondsHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_seconds",
		Help:    "Request latency",
		Buckets: []float64{0.1, 0.2, 0.5, 1.0, 2.0, 5.0, 10.0, 20, 50},
	}, []string{
		"method",
		"url",
		"status",
	})

	pm.requestBytesInCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "request_bytes_in",
		Help: "Bytes received from servers as response to requests",
	}, []string{
		"method",
		"url",
		"status",
	})

	pm.requestBytesOutCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "request_bytes_out",
		Help: "Bytes sent to servers during requests",
	}, []string{
		"method",
		"url",
		"status",
	})

	pm.requestFailCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "request_fail_count",
		Help: "Internal failures that prevented a hit to the target server",
	}, []string{
		"method",
		"url",
		"message",
	})

	//setup prometheus metrics http server
	router := mux.NewRouter()
	router.Handle(metricsPath, promhttp.Handler())

	listen := fmt.Sprintf("%s:%d", bindHost, bindPort)
	lp, err := net.Listen("tcp", listen)
	if err != nil {
		return PrometheusMetrics{}, err
	}
	pm.listenPort = lp

	go func() {
		http.Serve(pm.listenPort, router)
		defer pm.listenPort.Close()
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
	return pm.listenPort.Close()
}

//Observe register metrics about hit results
func (pm *PrometheusMetrics) Observe(res *vegeta.Result) {
	pm.requestBytesInCounter.WithLabelValues(res.Method, res.URL, fmt.Sprintf("%d", res.Code)).Add(float64(res.BytesIn))
	pm.requestBytesOutCounter.WithLabelValues(res.Method, res.URL, fmt.Sprintf("%d", res.Code)).Add(float64(res.BytesOut))
	pm.requestSecondsHistogram.WithLabelValues(res.Method, res.URL, fmt.Sprintf("%d", res.Code)).Observe(float64(res.Latency) / float64(time.Second))
	if res.Error != "" {
		pm.requestFailCounter.WithLabelValues(res.Method, res.URL, res.Error)
	}
}
