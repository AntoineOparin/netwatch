// Pure helpers for per-computer throughput: data shape, summaries, units.

export interface RateSample {
  at: number; // epoch milliseconds
  bytesPerSec: number;
}

export interface DeviceTraffic {
  iface: string;
  download: RateSample[];
  upload: RateSample[];
}

export type ThroughputData = Record<string, DeviceTraffic>; // by agent ID

export interface DeviceSummary {
  agentId: string;
  iface: string;
  // Latest rate, or null when the agent has gone quiet (offline, asleep).
  downNow: number | null;
  upNow: number | null;
  downAvg: number | null;
  upAvg: number | null;
}

// A rate older than this counts as "not reporting right now".
const STALE_MS = 6000;

export function summarize(
  agentId: string,
  traffic: DeviceTraffic,
  nowMs: number,
  windowMs: number,
): DeviceSummary {
  const latest = (s: RateSample[]) => {
    const last = s.at(-1);
    return last && nowMs - last.at <= STALE_MS ? last.bytesPerSec : null;
  };
  const avg = (s: RateSample[]) => {
    const recent = s.filter((p) => p.at >= nowMs - windowMs);
    return recent.length
      ? recent.reduce((sum, p) => sum + p.bytesPerSec, 0) / recent.length
      : null;
  };
  return {
    agentId,
    iface: traffic.iface,
    downNow: latest(traffic.download),
    upNow: latest(traffic.upload),
    downAvg: avg(traffic.download),
    upAvg: avg(traffic.upload),
  };
}

// ISPs sell speeds in megabits per second, so show that, not bytes.
export function formatRate(bytesPerSec: number | null) {
  if (bytesPerSec === null) return "—";
  const mbps = (bytesPerSec * 8) / 1_000_000;
  if (mbps >= 100) return `${Math.round(mbps)} Mbps`;
  return `${mbps.toFixed(1)} Mbps`;
}

export function toMbps(bytesPerSec: number) {
  return (bytesPerSec * 8) / 1_000_000;
}

// Colors follow the device, never its rank: slots are assigned by sorted
// agent ID, so a device keeps its color when others come and go.
const SLOTS = 8;
export function deviceColor(agentId: string, allIds: string[]) {
  const i = [...allIds].sort().indexOf(agentId);
  return i >= 0 && i < SLOTS ? `var(--series-${i + 1})` : "var(--chart-axis)";
}
