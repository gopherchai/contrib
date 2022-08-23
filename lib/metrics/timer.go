package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// namespace 统一用"t"(name.go中定义了常量) 或者其他产品
// metricName是指标名字，确保一个进程内唯一性
// help是描述指标用途
// labels 是维度

type timerOptions struct {
	buckets     []float64
	labels      map[string]string
	quantile    map[float64]float64
	needSummary bool
}

type TimerOption func(*timerOptions)

func WithTimerBuckets(buk []float64) TimerOption {
	return func(o *timerOptions) {
		o.buckets = buk
	}
}

func WithTimerQuantile(quantile map[float64]float64) TimerOption {
	return func(o *timerOptions) {
		o.quantile = quantile
	}
}

func WithTimerConstLabels(lables map[string]string) TimerOption {
	return func(o *timerOptions) {
		o.labels = lables
	}
}

func WithSummary(need bool) TimerOption {
	return func(o *timerOptions) {
		o.needSummary = need
	}
}

// NewTimer
func NewTimer(namespace, metricName, help string, labels []string, opts ...TimerOption) *Timer {

	timerOpts := timerOptions{
		needSummary: true,
	}
	for _, opt := range opts {
		opt(&timerOpts)
	}
	if len(timerOpts.quantile) == 0 {
		timerOpts.quantile = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
	}
	if len(timerOpts.buckets) == 0 {
		timerOpts.buckets = []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .5, 1, 2.5, 5, 10, 60, 600, 3600}
	}

	var summary *prometheus.SummaryVec
	if timerOpts.needSummary {
		// summary
		summary = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace:  namespace,
				Name:       metricName + "_s",
				Help:       help + " (summary)",
				Objectives: timerOpts.quantile,
			},
			labels)

		prometheus.MustRegister(summary)
	}

	// histogram
	hisOpts := prometheus.HistogramOpts{
		Namespace:   namespace,
		Name:        metricName + "_h",
		Help:        help + " (histogram)",
		Buckets:     timerOpts.buckets,
		ConstLabels: timerOpts.labels,
	}

	histogram := prometheus.NewHistogramVec(hisOpts, labels)

	prometheus.MustRegister(histogram)
	return &Timer{
		name:      metricName,
		summary:   summary,
		histogram: histogram,
	}
}

type Timer struct {
	name      string
	summary   *prometheus.SummaryVec
	histogram *prometheus.HistogramVec
}

// Timer 返回一个函数，并且开始计时，结束计时则调用返回的函数
// 请参考timer_test.go 的demo
func (t *Timer) Timer() func(values ...string) {
	if t == nil {
		return func(values ...string) {}
	}

	now := time.Now()

	return func(values ...string) {
		seconds := float64(time.Since(now)) / float64(time.Second)
		if t.summary != nil {
			t.summary.WithLabelValues(values...).Observe(seconds)
		}
		t.histogram.WithLabelValues(values...).Observe(seconds)
	}
}

// Observe ：传入duration和labels，
func (t *Timer) Observe(duration time.Duration, label ...string) {
	if t == nil {
		return
	}

	seconds := float64(duration) / float64(time.Second)
	if t.summary != nil {
		t.summary.WithLabelValues(label...).Observe(seconds)
	}
	t.histogram.WithLabelValues(label...).Observe(seconds)
}
