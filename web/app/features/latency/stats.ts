// Pure functions that turn raw probe results into the numbers and the
// one-line diagnosis the overview shows. No React here, so it's easy to test.

export const TARGETS = ["gateway", "isp", "internet"] as const;
export type Target = (typeof TARGETS)[number];

export const TARGET_LABELS: Record<Target, string> = {
  gateway: "Your router",
  isp: "ISP (first hop)",
  internet: "Internet",
};

// CSS variables from app.css, in fixed palette order.
export const TARGET_COLORS: Record<Target, string> = {
  gateway: "var(--series-1)",
  isp: "var(--series-2)",
  internet: "var(--series-3)",
};

export interface Sample {
  at: number; // epoch milliseconds
  rtt: number | null; // null = lost
}

export interface TargetSeries {
  addr: string;
  samples: Sample[];
}

export type LatencyData = Partial<Record<Target, TargetSeries>>;

export interface TargetStats {
  addr: string;
  last: number | null;
  avg: number | null;
  // Mean change between consecutive round trips. Games feel jitter more
  // than a steady high ping.
  jitter: number | null;
  loss: number; // 0..1
  count: number;
}

export function computeStats(
  series: TargetSeries,
  sinceMs: number,
): TargetStats {
  const recent = series.samples.filter((s) => s.at >= sinceMs);
  const rtts = recent.flatMap((s) => (s.rtt === null ? [] : [s.rtt]));
  let jitterSum = 0;
  for (let i = 1; i < rtts.length; i++) {
    jitterSum += Math.abs(rtts[i] - rtts[i - 1]);
  }
  return {
    addr: series.addr,
    last: recent.at(-1)?.rtt ?? null,
    avg: rtts.length ? rtts.reduce((a, b) => a + b, 0) / rtts.length : null,
    jitter: rtts.length > 1 ? jitterSum / (rtts.length - 1) : null,
    loss: recent.length ? (recent.length - rtts.length) / recent.length : 0,
    count: recent.length,
  };
}

export type Status = "unknown" | "good" | "warning" | "critical";

export interface Diagnosis {
  status: Status;
  title: string;
  detail: string;
}

// Thresholds per hop. Routers answer pings aimed at themselves at low
// priority, so the ISP hop is allowed to be noisier than the far target.
const LIMITS: Record<Target, { avg: number; jitter: number; loss: number }> = {
  gateway: { avg: 50, jitter: 30, loss: 0.05 },
  isp: { avg: 90, jitter: 45, loss: 0.1 },
  internet: { avg: 120, jitter: 40, loss: 0.03 },
};

function isBad(target: Target, s: TargetStats | undefined) {
  if (!s || s.count === 0) return false;
  const limit = LIMITS[target];
  return (
    s.loss >= limit.loss ||
    (s.avg ?? 0) >= limit.avg ||
    (s.jitter ?? 0) >= limit.jitter
  );
}

const ms = (v: number | null) => (v === null ? "—" : `${Math.round(v)} ms`);
const pct = (v: number) => `${Math.round(v * 100)}%`;
const summary = (s: TargetStats) =>
  `avg ${ms(s.avg)}, jitter ${ms(s.jitter)}, ${pct(s.loss)} lost`;

// diagnose walks the path outward: the first hop that looks bad is where
// the trouble starts, because every hop after it inherits the problem.
export function diagnose(
  stats: Partial<Record<Target, TargetStats>>,
): Diagnosis {
  const { gateway, isp, internet } = stats;
  if (!gateway?.count || !internet?.count) {
    return {
      status: "unknown",
      title: "Waiting for measurements…",
      detail: "The hub pings each hop once a second.",
    };
  }
  const severity = (s: TargetStats): Status =>
    s.loss >= 0.2 ? "critical" : "warning";

  if (isBad("gateway", gateway)) {
    return {
      status: severity(gateway),
      title: "Wi-Fi or local network trouble",
      detail: `Even your router is slow to answer (${summary(gateway)}). Everything past it suffers too. Try a cable, or move closer to the router.`,
    };
  }
  if (isp && isBad("isp", isp)) {
    return {
      status: severity(isp),
      title: "Your internet connection is congested",
      detail: `Your router is fine, but the first ISP router isn't (${summary(isp)}). Usually someone in the house is maxing out the upload or download, or the ISP's local node is busy.`,
    };
  }
  if (isBad("internet", internet)) {
    return {
      status: severity(internet),
      title: "Trouble past your ISP",
      detail: `Your router${isp ? " and ISP" : ""} look fine, but the internet target doesn't (${summary(internet)}). The problem is further out, not in your home.`,
    };
  }
  return {
    status: "good",
    title: "Connection looks healthy",
    detail: `Router ${ms(gateway.avg)}${isp ? ` · ISP ${ms(isp.avg)}` : ""} · internet ${ms(internet.avg)} average over the last minute.`,
  };
}
