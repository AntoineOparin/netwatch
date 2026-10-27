import { TimeSeriesChart } from "~/components/time-series-chart";
import {
  computeStats,
  type Diagnosis,
  diagnose,
  type Status,
  TARGET_COLORS,
  TARGET_LABELS,
  TARGETS,
  type TargetStats,
} from "./stats";
import { useLatencyStream } from "./use-latency-stream";

const STATS_WINDOW_MS = 60_000;

export function LatencyPanel({ windowMs }: { windowMs: number }) {
  const { data, connected } = useLatencyStream(windowMs);

  const newest = Math.max(
    0,
    ...TARGETS.map((t) => data[t]?.samples.at(-1)?.at ?? 0),
  );
  const stats: Partial<Record<(typeof TARGETS)[number], TargetStats>> = {};
  for (const t of TARGETS) {
    const series = data[t];
    if (series) stats[t] = computeStats(series, newest - STATS_WINDOW_MS);
  }
  const diagnosis = connected
    ? diagnose(stats)
    : {
        status: "unknown" as const,
        title: "Connecting to the hub…",
        detail: "Retrying every 2 seconds.",
      };
  const present = TARGETS.filter((t) => stats[t]);

  return (
    <section className="space-y-4">
      <Verdict diagnosis={diagnosis} />

      <div className="grid gap-3 sm:grid-cols-3">
        {present.map((t) => (
          <StatCard key={t} target={t} stats={stats[t] as TargetStats} />
        ))}
      </div>

      <div className="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900">
        <div className="mb-3">
          <h2 className="text-sm font-medium">Round-trip time</h2>
          <p className="text-xs text-gray-500">
            One ping per second to each hop. Red ticks are lost pings.
          </p>
        </div>
        <ul className="mb-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-600 dark:text-gray-400">
          {present.map((t) => (
            <li key={t} className="flex items-center gap-1.5">
              <span
                className="h-0.5 w-4 rounded-full"
                style={{ background: TARGET_COLORS[t] }}
              />
              {TARGET_LABELS[t]}
            </li>
          ))}
        </ul>
        <TimeSeriesChart
          series={present.map((t) => ({
            key: t,
            label: TARGET_LABELS[t],
            color: TARGET_COLORS[t],
            points: (data[t]?.samples ?? []).map((p) => ({
              at: p.at,
              value: p.rtt,
            })),
          }))}
          windowMs={windowMs}
          formatValue={(v) => `${Math.round(v)} ms`}
          yFloor={50}
          markMissing
        />
      </div>
    </section>
  );
}

const STATUS_STYLES: Record<Status, { icon: string; color: string }> = {
  unknown: { icon: "…", color: "var(--chart-axis)" },
  good: { icon: "✓", color: "var(--status-good)" },
  warning: { icon: "!", color: "var(--status-warning)" },
  critical: { icon: "✕", color: "var(--status-critical)" },
};

// Status is carried by an icon and a title, never by color alone.
function Verdict({ diagnosis }: { diagnosis: Diagnosis }) {
  const style = STATUS_STYLES[diagnosis.status];
  return (
    <div
      role="status"
      className="flex gap-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900"
    >
      <span
        aria-hidden
        className="flex size-7 shrink-0 items-center justify-center rounded-full text-sm font-bold text-white"
        style={{ background: style.color }}
      >
        {style.icon}
      </span>
      <div>
        <p className="font-semibold">{diagnosis.title}</p>
        <p className="mt-0.5 text-sm text-gray-600 dark:text-gray-400">
          {diagnosis.detail}
        </p>
      </div>
    </div>
  );
}

const fmt = (v: number | null) => (v === null ? "—" : v.toFixed(1));

function StatCard({
  target,
  stats,
}: {
  target: (typeof TARGETS)[number];
  stats: TargetStats;
}) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900">
      <div className="flex items-center gap-2 text-sm">
        <span
          className="h-0.5 w-4 rounded-full"
          style={{ background: TARGET_COLORS[target] }}
        />
        <span className="font-medium">{TARGET_LABELS[target]}</span>
        <span className="ml-auto font-mono text-xs text-gray-500">
          {stats.addr}
        </span>
      </div>
      <p className="mt-2 text-3xl font-semibold tabular-nums">
        {fmt(stats.avg)}
        <span className="text-base font-normal text-gray-500"> ms avg</span>
      </p>
      <dl className="mt-2 grid grid-cols-2 gap-1 text-xs text-gray-600 dark:text-gray-400">
        <dt>Jitter</dt>
        <dd className="text-right tabular-nums">{fmt(stats.jitter)} ms</dd>
        <dt>Lost</dt>
        <dd className="text-right tabular-nums">
          {Math.round(stats.loss * 100)}%
        </dd>
      </dl>
      <p className="mt-2 text-xs text-gray-400">Last minute</p>
    </div>
  );
}
