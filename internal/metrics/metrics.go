package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// WebSocket Metrics
	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "kaban_ws_active_connections",
		Help: "Current number of active WebSocket connections",
	})

	ActiveRooms = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "kaban_ws_active_rooms",
		Help: "Current number of active workspace rooms",
	})

	ConnectionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kaban_ws_connections_total",
		Help: "Total number of connection attempts",
	}, []string{"status"})

	DroppedClients = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kaban_ws_dropped_clients_total",
		Help: "Total clients dropped due to full send buffer",
	})

	MessagesBroadcasted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kaban_ws_messages_broadcasted_total",
		Help: "Total number of messages successfully sent to clients",
	})

	// Gateway Metrics
	EventsReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kaban_events_received_total",
		Help: "Total number of events received via HTTP Gateway",
	}, []string{"event_type"})

	// SSO/Auth Metrics
	SSOAuthDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "kaban_sso_auth_duration_seconds",
		Help:    "Time spent validating SSO tokens via gRPC",
		Buckets: prometheus.DefBuckets,
	})
)
