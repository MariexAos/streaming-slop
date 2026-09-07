export function snapshotFixture(revision = 1) {
  return {
    revision,
    observedAt: "2026-09-01T12:00:00Z",
    session: {
      id: "session-1",
      status: "running",
      startedAt: "2026-09-01T11:59:00Z",
      uptimeSeconds: 60,
      lastError: null,
    },
    buffer: {
      playheadSeconds: 10,
      commitHorizonSeconds: 30,
      readySeconds: 48,
      submittedSeconds: 86,
      readyTargetSeconds: 45,
      submittedTargetSeconds: 90,
    },
    timeline: [
      {
        id: "segment-3",
        sequence: 3,
        startSeconds: 10,
        endSeconds: 15,
        status: "playing",
        source: "generated",
      },
    ],
    generation: {
      mode: "normal",
      inFlight: 2,
      targetConcurrency: 3,
      latencyP50Seconds: 8,
      latencyP95Seconds: 12,
      failuresTotal: 1,
      failureRate: 0.05,
      succeededTotal: 3,
      costCny: 7.5,
      costPerLiveHourCny: 15,
      lastError: null,
    },
    stream: {
      status: "live",
      bitrateKbps: 3200,
      droppedFramesTotal: 0,
      gapTotal: 0,
      lastError: null,
    },
    fallback: { active: false, forced: false, reason: null, since: null, secondsTotal: 0 },
    controls: {
      canStart: false,
      canStop: true,
      canEnableFallback: true,
      canDisableFallback: false,
    },
  }
}
