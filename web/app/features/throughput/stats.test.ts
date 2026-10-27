import { describe, expect, test } from "bun:test";
import { deviceColor, formatRate, summarize } from "./stats";

describe("summarize", () => {
  const traffic = {
    iface: "en0",
    download: [
      { at: 0, bytesPerSec: 1_000_000 },
      { at: 50_000, bytesPerSec: 3_000_000 },
    ],
    upload: [{ at: 10_000, bytesPerSec: 500_000 }],
  };

  test("latest and averages", () => {
    const s = summarize("a", traffic, 52_000, 60_000);
    expect(s.downNow).toBe(3_000_000);
    expect(s.downAvg).toBe(2_000_000);
    expect(s.upAvg).toBe(500_000);
  });

  test("a quiet agent has no current rate", () => {
    expect(summarize("a", traffic, 52_000, 60_000).upNow).toBeNull();
  });
});

test("formatRate shows megabits", () => {
  expect(formatRate(1_250_000)).toBe("10.0 Mbps");
  expect(formatRate(25_000_000)).toBe("200 Mbps");
  expect(formatRate(null)).toBe("—");
});

test("a device keeps its color when others join", () => {
  expect(deviceColor("b", ["b", "c"])).toBe("var(--series-1)");
  expect(deviceColor("c", ["c", "b"])).toBe("var(--series-2)");
  expect(deviceColor("c", ["a", "b", "c"])).toBe("var(--series-3)");
});
