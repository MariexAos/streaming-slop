import { snapshotFixture } from "./snapshot.fixture"
import { servicesFixture } from "./services.fixture"

export function budgetJourneyFixture() {
  const snapshot = snapshotFixture()
  return {
    snapshot: {
      ...snapshot,
      session: { ...snapshot.session, status: "stopped" },
      controls: { ...snapshot.controls, canStart: true, canStop: false },
    },
    services: servicesFixture(),
    budget: {
      nextLimitMicros: 10000000,
      limitMicros: 10000000,
      chargedMicros: 0,
      reservedMicros: 0,
    },
    readiness: {
      ready: true,
      target: "本地测试",
      characterId: "host",
      characterName: "主播",
      credentialSaved: true,
      availableMicros: 10000000,
      minimumMicros: 7550000,
      blockers: [],
    },
  }
}
