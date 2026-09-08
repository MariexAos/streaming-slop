import { expect, test } from "vite-plus/test"
import { snapshotFixture } from "./snapshot.fixture"
import { opsSnapshotSchema } from "./schema"
import { journeyState } from "./journey"
test("preparing is distinct from live even when stop is available", () => {
  const snapshot = opsSnapshotSchema.parse(snapshotFixture())
  snapshot.session.status = "buffering"
  snapshot.controls.canStop = true
  expect(journeyState(snapshot)).toMatchObject({ live: false, preparing: true })
  snapshot.session.status = "running"
  snapshot.stream.status = "live"
  expect(journeyState(snapshot)).toMatchObject({ live: true, preparing: false })
  snapshot.session.status = "stopping"
  expect(journeyState(snapshot)).toMatchObject({ live: false, preparing: false, active: true })
})
