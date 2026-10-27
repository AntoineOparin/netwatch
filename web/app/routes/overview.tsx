import { useState } from "react";
import { Link } from "react-router";
import { InfoTip } from "~/components/info-tip";
import { PageHeader } from "~/components/page-header";
import { WINDOWS, WindowPicker } from "~/components/window-picker";
import {
  incidentRanges,
  RecentIncidents,
} from "~/features/incidents/incident-list";
import {
  HopCards,
  LatencyChart,
  useLatency,
  Verdict,
} from "~/features/latency/latency-panel";
import type { TermId } from "~/features/learn/glossary";
import { ThroughputPanel } from "~/features/throughput/throughput-panel";
import { summarizeUptime } from "~/features/uptime/format";
import { dashboard, usePolling } from "~/lib/api";
import type { Route } from "./+types/overview";

const MAX_WINDOW_SECONDS = BigInt(WINDOWS[WINDOWS.length - 1].ms / 1000);

export function meta() {
  return [{ title: "Overview · NetWatch" }];
}

export async function clientLoader() {
  const [{ agents }, { incidents }, { devices }, { apps }, uptime] =
    await Promise.all([
      dashboard().listAgents({}),
      dashboard().listIncidents({ since: { seconds: MAX_WINDOW_SECONDS } }),
      dashboard().listDevices({}),
      dashboard().listTopApps({ window: { seconds: 60n } }),
      dashboard().getUptime({ window: { seconds: 86_400n } }),
    ]);
  return { agents, incidents, devices, apps, uptime: summarizeUptime(uptime) };
}

export default function Overview({ loaderData }: Route.ComponentProps) {
  usePolling(5000);
  const [windowMs, setWindowMs] = useState(WINDOWS[1].ms);
  const latency = useLatency(windowMs);
  const { agents, incidents, devices, apps, uptime } = loaderData;
  const wifi = Object.fromEntries(agents.map((a) => [a.id, a.wifi]));

  const now = Date.now();
  const hostnames = Object.fromEntries(agents.map((a) => [a.id, a.hostname]));
  const agentOS = Object.fromEntries(agents.map((a) => [a.id, a.os]));
  const highlights = incidentRanges(incidents, now);
  const incidentsInWindow = incidents.filter(
    (i) => !i.end || Number(i.end.seconds) * 1000 >= now - windowMs,
  ).length;

  return (
    <>
      <PageHeader
        title="Overview"
        description="A live view of your connection: where any lag starts, when it happened, and who was using the internet at the time."
        actions={<WindowPicker value={windowMs} onChange={setWindowMs} />}
      />

      <div className="space-y-6">
        <div className="grid gap-6 xl:grid-cols-3">
          <div className="xl:col-span-2">
            <Verdict diagnosis={latency.diagnosis} />
          </div>
          <AtAGlance
            devicesOnline={devices.filter((d) => d.online).length}
            agentsOnline={agents.filter((a) => a.online).length}
            agents={agents.length}
            incidents={incidentsInWindow}
            uptime={uptime.percent}
          />
        </div>

        <HopCards latency={latency} />

        <div className="grid gap-6 xl:grid-cols-3">
          <div className="min-w-0 xl:col-span-2">
            <LatencyChart
              latency={latency}
              windowMs={windowMs}
              highlights={highlights}
            />
          </div>
          <RecentIncidents
            incidents={incidents}
            hostnames={hostnames}
            windowMs={windowMs}
          />
        </div>

        <div className="pt-4">
          <ThroughputPanel
            windowMs={windowMs}
            hostnames={hostnames}
            agentOS={agentOS}
            apps={apps}
            wifi={wifi}
            highlights={highlights}
          />
        </div>
      </div>
    </>
  );
}

function AtAGlance({
  devicesOnline,
  agentsOnline,
  agents,
  incidents,
  uptime,
}: {
  devicesOnline: number;
  agentsOnline: number;
  agents: number;
  incidents: number;
  uptime: string;
}) {
  const rows: {
    label: string;
    term: TermId;
    value: React.ReactNode;
    to: string;
  }[] = [
    {
      label: "Devices on your network",
      term: "lan",
      value: devicesOnline,
      to: "/devices",
    },
    {
      label: "Computers reporting",
      term: "agent",
      value: (
        <>
          {agentsOnline}
          <span className="text-zinc-400"> / {agents}</span>
        </>
      ),
      to: "/devices",
    },
    {
      label: "Internet uptime, 24 h",
      term: "uptime",
      value: uptime,
      to: "/incidents",
    },
    {
      label: "Lag incidents in view",
      term: "incident",
      value: incidents,
      to: "/incidents",
    },
  ];
  return (
    <section aria-label="At a glance" className="panel px-5 py-2">
      <dl className="divide-y divide-zinc-950/[0.06] dark:divide-white/[0.06]">
        {rows.map((r) => (
          <div key={r.label} className="flex items-center justify-between py-3">
            <dt className="flex items-center gap-1 text-sm text-zinc-600 dark:text-zinc-400">
              <Link
                to={r.to}
                className="hover:text-zinc-900 hover:underline dark:hover:text-zinc-100"
              >
                {r.label}
              </Link>
              <InfoTip id={r.term} />
            </dt>
            <dd className="text-lg font-semibold tabular-nums">{r.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
