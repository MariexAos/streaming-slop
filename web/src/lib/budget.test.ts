import { describe, expect, it } from "vite-plus/test"
import { budgetMicros } from "./budget"

describe("per-session budget input", () => {
  it("accepts currency amounts and rejects invalid or unsafe values", () => {
    expect(budgetMicros("10")).toBe(10000000)
    expect(budgetMicros("0.01")).toBe(10000)
    for (const value of ["", "0", "-1", "1.001", "NaN", "Infinity", "1e6", "9007199254740992"]) {
      expect(budgetMicros(value)).toBeUndefined()
    }
  })
})
