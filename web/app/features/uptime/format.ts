import type { GetUptimeResponse } from "~/gen/netwatch/v1/dashboard_pb";

const seconds = (d?: { seconds: bigint; nanos: number }) =>
  d ? Number(d.seconds) + d.nanos / 1e9 : 0;

export interface UptimeSummary {
  // "100%", "99.97%", or "—" when nothing was watched yet.
  percent: string;
  outages: number;
  downtimeS: number;
  longestS: number;
  monitoredS: number;
}

export function summarizeUptime(u: GetUptimeResponse): UptimeSummary {
  const monitoredS = seconds(u.monitored);
  const downtimeS = seconds(u.downtime);
  let percent = "—";
  if (monitoredS > 0) {
    const ratio = 1 - downtimeS / monitoredS;
    // Round down, so any outage at all shows as less than 100%.
    percent =
      ratio >= 1 ? "100%" : `${(Math.floor(ratio * 10000) / 100).toFixed(2)}%`;
  }
  return {
    percent,
    outages: u.outages,
    downtimeS,
    longestS: seconds(u.longestOutage),
    monitoredS,
  };
}
