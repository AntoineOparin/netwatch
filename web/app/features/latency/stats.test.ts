import { describe, expect, test } from "bun:test";
import { computeStats, diagnose, type Sample, type TargetStats } from "./stats";

const series = (rtts: (number | null)[]) => ({
  addr: "x",
  samples: rtts.map((rtt, i): Sample => ({ at: i * 1000, rtt })),
});

describe("computeStats", () => {
  test("averages, jitter and loss", () => {
    const s = computeStats(series([10, 20, null, 10]), 0);
    expect(s.avg).toBeCloseTo(13.33, 1);
    expect(s.jitter).toBe(10); // |20-10| and |10-20|, lost ping skipped
    expect(s.loss).toBe(0.25);
    expect(s.last).toBe(10);
  });

  test("ignores samples before the window", () => {
    expect(computeStats(series([500, 10, 10]), 1000).avg).toBe(10);
  });
});

const healthy = (avg: number): TargetStats => ({
  addr: "x",
  last: avg,
  avg,
  jitter: 2,
  loss: 0,
  count: 60,
});

describe("diagnose", () => {
  test("waits for data", () => {
    expect(diagnose({}).status).toBe("unknown");
  });

  test("healthy path", () => {
    const d = diagnose({
      gateway: healthy(4),
      isp: healthy(20),
      internet: healthy(25),
    });
    expect(d.status).toBe("good");
  });

  test("blames the first bad hop: the router", () => {
    const d = diagnose({
      gateway: { ...healthy(80), jitter: 40 },
      isp: healthy(200),
      internet: healthy(220),
    });
    expect(d.title).toMatch(/Wi-Fi/);
  });

  test("router fine, ISP hop bad: congestion", () => {
    const d = diagnose({
      gateway: healthy(4),
      isp: healthy(180),
      internet: healthy(190),
    });
    expect(d.title).toMatch(/congested/);
  });

  test("heavy loss is critical", () => {
    const d = diagnose({
      gateway: healthy(4),
      isp: healthy(20),
      internet: { ...healthy(25), loss: 0.5 },
    });
    expect(d.status).toBe("critical");
    expect(d.title).toMatch(/past your ISP/);
  });
});
