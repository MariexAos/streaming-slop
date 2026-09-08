import type { ModelServices } from "./services-api"
import { turboConfigFixture } from "./pricing.fixture"

export function servicesFixture(): ModelServices {
  const saved: ModelServices["saved"] = {
    video: { provider: "minimax", model: "MiniMax-H3-Max", resolution: "480P" },
    text: { provider: "minimax", model: "MiniMax-M3" },
  }
  return {
    active: saved,
    saved,
    credentials: [
      { provider: "minimax", configured: true },
      { provider: "fal", configured: true },
    ],
    options: [
      {
        provider: "minimax",
        model: "MiniMax-H3-Max",
        kind: "video",
        resolutions: ["480P", "768P"],
        prices: [],
      },
      {
        provider: "fal",
        model: "minimax/h3-max-turbo/image-to-video",
        kind: "video",
        resolutions: ["480P", "768P"],
        prices: turboConfigFixture.pricing ? [turboConfigFixture.pricing] : [],
      },
      { provider: "minimax", model: "MiniMax-M3", kind: "text", resolutions: [], prices: [] },
    ],
  }
}
