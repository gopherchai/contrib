package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type HistogramVec struct {
	histogram *prometheus.HistogramVec
}

type histogramOptions struct {
	buckets []float64
	labels  map[string]string
}

type HistogramOption func(*histogramOptions)

func WithBuckets(buk []float64) HistogramOption {
	return func(o *histogramOptions) {
		o.buckets = buk
	}
}

func WithLabels(lables map[string]string) HistogramOption {
	return func(o *histogramOptions) {
		o.labels = lables
	}
}

func NewHistogramVec(namespace, subSystem, metricsName, help string, labels []string, opts ...HistogramOption) *HistogramVec {
	histogramOpts := histogramOptions{}
	for _, opt := range opts {
		opt(&histogramOpts)
	}
	if len(histogramOpts.buckets) == 0 {
		histogramOpts.buckets = []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .5, 1, 2.5, 5, 10, 60, 600, 3600}
	}
	hisOpts := prometheus.HistogramOpts{
		Namespace:   namespace,
		Subsystem:   subSystem,
		Name:        metricsName + "_h",
		Help:        help + " (histogram)",
		Buckets:     histogramOpts.buckets,
		ConstLabels: histogramOpts.labels,
	}

	histogram := prometheus.NewHistogramVec(hisOpts, labels)

	prometheus.MustRegister(histogram)

	return &HistogramVec{
		histogram: histogram,
	}
}

func (h *HistogramVec) Observe(value float64, labels ...string) {
	h.histogram.WithLabelValues(labels...).Observe(value)
}
