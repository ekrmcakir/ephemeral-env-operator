package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	environmentsCreatedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ephemeral_env_created_total",
			Help: "Total count of ephemeral preview environments created",
		},
		[]string{"environment"},
	)

	environmentsDestroyedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ephemeral_env_destroyed_total",
			Help: "Total count of ephemeral preview environments destroyed via TTL or PR closure",
		},
		[]string{"environment"},
	)

	activeEnvironmentsGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ephemeral_env_active_gauge",
			Help: "Current count of active ephemeral preview environments running in cluster",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(
		environmentsCreatedTotal,
		environmentsDestroyedTotal,
		activeEnvironmentsGauge,
	)
}

// RecordEnvironmentCreated increments creation counter and active gauge
func RecordEnvironmentCreated(name string) {
	environmentsCreatedTotal.WithLabelValues(name).Inc()
	activeEnvironmentsGauge.Inc()
}

// RecordEnvironmentDestroyed increments destruction counter and decrements active gauge
func RecordEnvironmentDestroyed(name string) {
	environmentsDestroyedTotal.WithLabelValues(name).Inc()
	activeEnvironmentsGauge.Dec()
}
