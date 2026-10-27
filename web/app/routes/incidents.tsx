import { useState } from "react";
import { Term } from "~/components/info-tip";
import { PageHeader } from "~/components/page-header";
import { type WindowOption, WindowPicker } from "~/components/window-picker";
import { formatDuration } from "~/features/incidents/explain";
import { IncidentList } from "~/features/incidents/incident-list";
import { summarizeUptime, type UptimeSummary } from "~/features/uptime/format";
import { dashboard, usePolling } from "~/lib/api";
import type { Route } from "./+types/incidents";

const HOUR = 60 * 60_000;
const WINDOWS: WindowOption[] = [
  { label: "1 hour", ms: HOUR },
  { label: "24 hours", ms: 24 * HOUR },
  { label: "7 days", ms: 7 * 24 * HOUR },
  { label: "30 days", ms: 30 * 24 * HOUR },
];
const MAX_SECONDS = BigInt(WINDOWS[WINDOWS.length - 1].ms / 1000);

export function meta() {
  return [{ title: "Lag incidents · NetWatch" }];
}

export async function clientLoader() {
  const [{ incidents }, { agents }, ...uptimes] = await Promise.all([
    dashboard().listIncidents({ since: { seconds: MAX_SECONDS } }),
    dashboard().listAgents({}),
    ...WINDOWS.map((w) =>
      dashboard().getUptime({ window: { seconds: BigInt(w.ms / 1000) } }),
    ),
  ]);
  // One uptime summary per window, keyed by its length.
  const uptime = Object.fromEntries(
    WINDOWS.map((w, i) => [w.ms, summarizeUptime(uptimes[i])]),
  );
  return { incidents, agents, uptime };
}

export default function Incidents({ loaderData }: Route.ComponentProps) {
  usePolling(10_000);
  const [windowMs, setWindowMs] = useState(WINDOWS[1].ms);
  const { incidents, agents, uptime } = loaderData;
  const hostnames = Object.fromEntries(agents.map((a) => [a.id, a.hostname]));

  return (
    <>
      <PageHeader
        title="Lag incidents"
        description={
          <>
            Every time internet ping rose well above its{" "}
            <Term id="baseline">normal level</Term>, with where it started and
            which computers were <Term id="suspect">busy at the same time</Term>
            .
          </>
        }
        actions={
          <WindowPicker
            value={windowMs}
            onChange={setWindowMs}
            options={WINDOWS}
          />
        }
      />
      <UptimeStrip summary={uptime[windowMs]} />
      <IncidentList
        incidents={incidents}
        hostnames={hostnames}
        windowMs={windowMs}
      />
    </>
  );
}

// Uptime, outages and downtime for the selected window.
function UptimeStrip({ summary }: { summary: UptimeSummary }) {
  const stats = [
    {
      id: "uptime",
      label: <Term id="uptime">Internet uptime</Term>,
      value: summary.percent,
    },
    {
      id: "outages",
      label: <Term id="outage">Outages</Term>,
      value: summary.outages,
    },
    {
      id: "downtime",
      label: "Total downtime",
      value: summary.downtimeS
        ? formatDuration(summary.downtimeS * 1000)
        : "None",
    },
    {
      id: "longest",
      label: "Longest outage",
      value: summary.longestS ? formatDuration(summary.longestS * 1000) : "—",
    },
  ];
  return (
    <section
      aria-label="Uptime"
      className="panel mb-6 grid grid-cols-2 divide-zinc-950/[0.06] sm:grid-cols-4 sm:divide-x dark:divide-white/[0.06]"
    >
      {stats.map((s) => (
        <div key={s.id} className="px-5 py-4">
          <p className="eyebrow">{s.label}</p>
          <p className="mt-1 text-2xl font-semibold tracking-tight tabular-nums">
            {s.value}
          </p>
        </div>
      ))}
      <p className="col-span-full border-t border-zinc-950/[0.06] px-5 py-2.5 text-xs text-zinc-500 dark:border-white/[0.06]">
        Counted over the {formatDuration(summary.monitoredS * 1000)} the hub was
        running in this window.
      </p>
    </section>
  );
}
