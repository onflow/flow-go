package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Registerer struct {
	prometheus.Registerer
}

func NewRegisterer(registerer prometheus.Registerer) *Registerer {
	return &Registerer{registerer}
}

func (r *Registerer) RegisterNewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram {
	histogram := prometheus.NewHistogram(opts)
	r.MustRegister(histogram)
	return histogram
}

func (r *Registerer) RegisterNewHistogramVec(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec {
	histogram := prometheus.NewHistogramVec(opts, labelNames)
	r.MustRegister(histogram)
	return histogram
}

func (r *Registerer) RegisterNewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	counter := prometheus.NewCounter(opts)
	r.MustRegister(counter)
	return counter
}

func (r *Registerer) RegisterNewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec {
	counter := prometheus.NewCounterVec(opts, labelNames)
	r.MustRegister(counter)
	return counter
}

func (r *Registerer) RegisterNewGauge(opts prometheus.GaugeOpts) prometheus.Gauge {
	gauge := prometheus.NewGauge(opts)
	r.MustRegister(gauge)
	return gauge
}

func (r *Registerer) RegisterNewGaugeVec(opts prometheus.GaugeOpts, labelNames []string) *prometheus.GaugeVec {
	gauge := prometheus.NewGaugeVec(opts, labelNames)
	r.MustRegister(gauge)
	return gauge
}
