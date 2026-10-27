import { InfoTip, Term } from "~/components/info-tip";
import { SectionHeader } from "~/components/page-header";
import {
  type Highlight,
  TimeSeriesChart,
} from "~/components/time-series-chart";
import type { TermId } from "~/features/learn/glossary";
import {
  computeStats,
  type Diagnosis,
  diagnose,
  type LatencyData,
  type Status,
  TARGET_COLORS,
  TARGET_LABELS,
  TARGETS,
  type Target,
  type TargetStats,
} from "./stats";
import { useLatencyStream } from "./use-latency-stream";

const STATS_WINDOW_MS = 60_000;

const TARGET_TERMS: Record<Target, TermId> = {
  gateway: "router",
  isp: "isp",
  internet: "internet",
};

export interface Latency {
  data: LatencyData;
  stats: Partial<Record<Target, TargetStats>>;
  present: Target[];
  diagnosis: Diagnosis;
}

// The live probe data plus everything derived from it, for the overview's
// verdict, hop cards and chart.
export function useLatency(windowMs: number): Latency {
  const { data, connected } = useLatencyStream(windowMs);
  const newest = Math.max(
    0,
    ...TARGETS.map((t) => data[t]?.samples.at(-1)?.at ?? 0),
  );
  const stats: Partial<Record<Target, TargetStats>> = {};
  for (const t of TARGETS) {
    const series = data[t];
    if (series) stats[t] = computeStats(series, newest - STATS_WINDOW_MS);
  }
  const diagnosis: Diagnosis = connected
    ? diagnose(stats)
    : {
        status: "unknown",
        title: "Connecting to the hub…",
        detail: "Retrying every 2 seconds.",
      };
  return {
    data,
    stats,
    present: TARGETS.filter((t) => stats[t]),
    diagnosis,
  };
}

const STATUS_STYLES: Record<Status, { icon: string; color: string }> = {
  unknown: { icon: "…", color: "var(--chart-axis)" },
  good: { icon: "✓", color: "var(--status-good)" },
  warning: { icon: "!", color: "var(--status-warning)" },
  critical: { icon: "✕", color: "var(--status-critical)" },
};

// The one-sentence answer at the top of the page. Status is carried by an
// icon and a title, never by color alone.
export function Verdict({ diagnosis }: { diagnosis: Diagnosis }) {
  const style = STATUS_STYLES[diagnosis.status];
  return (
    <section
      role="status"
      aria-label="Connection status"
      className="panel flex h-full items-center gap-4 p-5 sm:p-6"
    >
      <span
        aria-hidden
        className="flex size-10 shrink-0 items-center justify-center rounded-xl text-lg font-bold text-white"
        style={{ background: style.color }}
      >
        {style.icon}
      </span>
      <div className="min-w-0">
        <p className="eyebrow">Right now</p>
        <p className="mt-0.5 text-xl font-semibold tracking-tight">
          {diagnosis.title}
        </p>
        <p className="mt-1 max-w-2xl text-sm text-zinc-600 dark:text-zinc-400">
          {diagnosis.detail}
        </p>
      </div>
    </section>
  );
}

const fmt = (v: number | null) => (v === null ? "—" : v.toFixed(1));

// One card per hop: router, ISP, internet.
export function HopCards({ latency }: { latency: Latency }) {
  if (latency.present.length === 0) return null;
  return (
    <section
      aria-label="Ping to each hop"
      className="grid gap-4 sm:grid-cols-3"
    >
      {latency.present.map((t) => (
        <HopCard key={t} target={t} stats={latency.stats[t] as TargetStats} />
      ))}
    </section>
  );
}

function HopCard({ target, stats }: { target: Target; stats: TargetStats }) {
  return (
    <div className="panel p-5">
      <div className="flex items-center gap-2 text-sm">
        <span
          aria-hidden
          className="h-0.5 w-4 rounded-full"
          style={{ background: TARGET_COLORS[target] }}
        />
        <Term id={TARGET_TERMS[target]}>
          <span className="font-medium">{TARGET_LABELS[target]}</span>
        </Term>
        <span className="ml-auto font-mono text-xs text-zinc-400">
          {stats.addr}
        </span>
      </div>
      <p className="mt-3 text-3xl font-semibold tracking-tight tabular-nums">
        {fmt(stats.avg)}
        <span className="ml-1 text-sm font-normal text-zinc-500">ms ping</span>
      </p>
      <dl className="mt-3 grid grid-cols-[1fr_auto] items-center gap-y-1 border-t border-zinc-950/[0.06] pt-3 text-xs text-zinc-600 dark:border-white/[0.06] dark:text-zinc-400">
        <dt>
          <Term id="jitter">Jitter</Term>
        </dt>
        <dd className="text-right tabular-nums">{fmt(stats.jitter)} ms</dd>
        <dt>
          <Term id="loss">Lost</Term>
        </dt>
        <dd className="text-right tabular-nums">
          {Math.round(stats.loss * 100)}%
        </dd>
      </dl>
      <p className="mt-2 text-[11px] text-zinc-400">
        Average over the last minute
      </p>
    </div>
  );
}

// Ping to each hop over the selected window, with lag incidents shaded.
export function LatencyChart({
  latency,
  windowMs,
  highlights,
}: {
  latency: Latency;
  windowMs: number;
  highlights: Highlight[];
}) {
  const { data, present } = latency;
  return (
    <section className="panel p-5">
      <SectionHeader
        title={
          <span className="inline-flex items-center gap-1">
            Ping over time <InfoTip id="hops" />
          </span>
        }
        description="One ping per second to each hop. Red ticks are lost pings; shaded stretches are lag incidents."
      />
      <ul className="mb-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-600 dark:text-zinc-400">
        {present.map((t) => (
          <li key={t} className="flex items-center gap-1.5">
            <span
              aria-hidden
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
        highlights={highlights}
        height={280}
      />
    </section>
  );
}
