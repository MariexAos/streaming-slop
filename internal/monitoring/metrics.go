package monitoring

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	readySeconds      prometheus.Gauge
	submittedSeconds  prometheus.Gauge
	generationLatency *prometheus.GaugeVec
	generationFailure prometheus.Counter
	generationCost    prometheus.Counter
	fallbackSeconds   prometheus.Counter
	droppedFrames     prometheus.Counter
	streamBitrate     prometheus.Gauge

	lastFailures uint64
	lastCost     float64
	lastFallback float64
	lastDropped  uint64
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		readySeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "live_ready_seconds", Help: "Contiguous playable media ahead of the playhead.",
		}),
		submittedSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "live_submitted_seconds", Help: "Contiguous submitted media ahead of the playhead.",
		}),
		generationLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "generation_latency_seconds", Help: "Rolling generation latency quantiles.",
		}, []string{"quantile_source"}),
		generationFailure: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "generation_failure_total", Help: "Terminal generation failures.",
		}),
		generationCost: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "generation_cost_cny", Help: "Known video generation cost in CNY.",
		}),
		fallbackSeconds: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fallback_seconds_total", Help: "Seconds streamed from fallback media.",
		}),
		droppedFrames: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "stream_dropped_frames_total", Help: "Frames dropped by the stream output.",
		}),
		streamBitrate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "stream_bitrate", Help: "Current stream bitrate in kilobits per second.",
		}),
	}
	registerer.MustRegister(
		m.readySeconds, m.submittedSeconds, m.generationLatency,
		m.generationFailure, m.generationCost, m.fallbackSeconds,
		m.droppedFrames, m.streamBitrate,
	)
	return m
}

func (m *Metrics) ObserveSnapshot(snapshot Snapshot) {
	m.readySeconds.Set(snapshot.Buffer.ReadySeconds)
	m.submittedSeconds.Set(snapshot.Buffer.SubmittedSeconds)
	if snapshot.Generation.LatencyP50Seconds != nil {
		m.generationLatency.WithLabelValues("p50").Set(*snapshot.Generation.LatencyP50Seconds)
	}
	if snapshot.Generation.LatencyP95Seconds != nil {
		m.generationLatency.WithLabelValues("p95").Set(*snapshot.Generation.LatencyP95Seconds)
	}
	if snapshot.Generation.FailuresTotal > m.lastFailures {
		m.generationFailure.Add(float64(snapshot.Generation.FailuresTotal - m.lastFailures))
	}
	m.lastFailures = snapshot.Generation.FailuresTotal
	if snapshot.Generation.CostCNY != nil && *snapshot.Generation.CostCNY > m.lastCost {
		m.generationCost.Add(*snapshot.Generation.CostCNY - m.lastCost)
		m.lastCost = *snapshot.Generation.CostCNY
	}
	if snapshot.Fallback.SecondsTotal > m.lastFallback {
		m.fallbackSeconds.Add(snapshot.Fallback.SecondsTotal - m.lastFallback)
	}
	m.lastFallback = snapshot.Fallback.SecondsTotal
	if snapshot.Stream.DroppedFramesTotal > m.lastDropped {
		m.droppedFrames.Add(float64(snapshot.Stream.DroppedFramesTotal - m.lastDropped))
	}
	m.lastDropped = snapshot.Stream.DroppedFramesTotal
	if snapshot.Stream.BitrateKbps != nil {
		m.streamBitrate.Set(*snapshot.Stream.BitrateKbps)
	}
}
