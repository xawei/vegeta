package prom

import (
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func TestPromServerBasic1(t *testing.T) {
	pm, err := NewPrometheusMetrics()
	assert.Nil(t, err, "Error launching Prometheus http server. err=%s", err)
	err = pm.Close()
	assert.Nil(t, err, "Error stopping Prometheus http server. err=%s", err)
}

func TestPromServerBasic2(t *testing.T) {
	pm, err := NewPrometheusMetrics()
	assert.Nil(t, err, "Error launching Prometheus metrics. err=%s", err)
	err = pm.Close()
	assert.Nil(t, err, "Error stopping Prometheus http server. err=%s", err)

	pm, err = NewPrometheusMetrics()
	assert.Nil(t, err, "Error launching Prometheus metrics. err=%s", err)
	err = pm.Close()
	assert.Nil(t, err, "Error stopping Prometheus http server. err=%s", err)

	pm, err = NewPrometheusMetrics()
	assert.Nil(t, err, "Error launching Prometheus metrics. err=%s", err)
	err = pm.Close()
	assert.Nil(t, err, "Error stopping Prometheus http server. err=%s", err)
}

func TestPromServerObserve(t *testing.T) {
	pm, err := NewPrometheusMetrics()
	assert.Nil(t, err, "Error launching Prometheus http server. err=%s", err)

	r := &vegeta.Result{
		URL:      "http://test.com/test1",
		Method:   "GET",
		Code:     200,
		Error:    "",
		Latency:  100 * time.Millisecond,
		BytesIn:  1000,
		BytesOut: 50,
	}
	pm.Observe(r)
	pm.Observe(r)
	pm.Observe(r)
	pm.Observe(r)

	time.Sleep(1 * time.Second)
	resp, err := http.Get("http://localhost:8880")
	assert.Nil(t, err, "Error calling prometheus metrics. err=%s", err)
	assert.Equal(t, 200, resp.StatusCode, "Status code should be 200")

	data, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err, "Error calling prometheus metrics. err=%s", err)
	str := string(data)
	assert.NotEqual(t, 0, len(str), "Body not empty")
	assert.Contains(t, str, "request_seconds", "Metrics should contain request_seconds")
	assert.Contains(t, str, "request_bytes_in", "Metrics should contain request_bytes_in")
	assert.Contains(t, str, "request_bytes_out", "Metrics should contain request_bytes_out")
	assert.NotContains(t, str, "request_fail_count", "Metrics should contain request_fail_count")

	r.Code = 500
	r.Error = "REQUEST FAILED"
	pm.Observe(r)

	resp, err = http.Get("http://localhost:8880")
	assert.Nil(t, err, "Error calling prometheus metrics. err=%s", err)
	assert.Equal(t, 200, resp.StatusCode, "Status code should be 200")

	data, err = ioutil.ReadAll(resp.Body)
	assert.Nil(t, err, "Error calling prometheus metrics. err=%s", err)
	str = string(data)

	assert.Contains(t, str, "request_fail_count", "Metrics should contain request_fail_count")

	err = pm.Close()
	assert.Nil(t, err, "Error stopping Prometheus http server. err=%s", err)
}
