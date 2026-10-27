import { useState } from "react";
import { Link } from "react-router";
import { WINDOWS, WindowPicker } from "~/components/window-picker";
import {
  IncidentList,
  incidentRanges,
} from "~/features/incidents/incident-list";
import { LatencyPanel } from "~/features/latency/latency-panel";
import { ThroughputPanel } from "~/features/throughput/throughput-panel";
import { dashboard, usePolling } from "~/lib/api";
import type { Route } from "./+types/overview";

const MAX_WINDOW_SECONDS = BigInt(WINDOWS[WINDOWS.length - 1].ms / 1000);

export async function clientLoader() {
  const [{ agents }, { incidents }] = await Promise.all([
    dashboard().listAgents({}),
    dashboard().listIncidents({ since: { seconds: MAX_WINDOW_SECONDS } }),
  ]);
  return { agents, incidents };
}

export default function Overview({ loaderData }: Route.ComponentProps) {
  usePolling(5000);
  const [windowMs, setWindowMs] = useState(WINDOWS[1].ms);
  const { agents, incidents } = loaderData;
  const online = agents.filter((a) => a.online).length;
  const hostnames = Object.fromEntries(agents.map((a) => [a.id, a.hostname]));
  const highlights = incidentRanges(incidents, Date.now());

  return (
    <main className="space-y-8">
      <WindowPicker value={windowMs} onChange={setWindowMs} />
      <LatencyPanel windowMs={windowMs} highlights={highlights} />
      <IncidentList
        incidents={incidents}
        hostnames={hostnames}
        windowMs={windowMs}
      />
      <ThroughputPanel
        windowMs={windowMs}
        hostnames={hostnames}
        highlights={highlights}
      />

      <div className="grid gap-4 sm:grid-cols-2">
        <section className="rounded-xl border border-gray-200 bg-white p-5 dark:border-gray-800 dark:bg-gray-900">
          <h2 className="text-sm font-medium text-gray-500">Agents online</h2>
          <p className="mt-2 text-4xl font-semibold tabular-nums">
            {online}
            <span className="text-xl text-gray-400"> / {agents.length}</span>
          </p>
          <Link
            to="/devices"
            className="mt-3 inline-block text-sm text-blue-600 hover:underline dark:text-blue-400"
          >
            See devices →
          </Link>
        </section>

        {agents.length === 0 && (
          <section className="rounded-xl border border-dashed border-gray-300 p-5 text-sm dark:border-gray-700">
            <h2 className="font-medium">No agents yet</h2>
            <p className="mt-2 text-gray-500">
              On each computer, run the agent pointed at this hub:
            </p>
            <pre className="mt-2 overflow-x-auto rounded bg-gray-100 p-2 dark:bg-gray-800">
              <code>agent -hub {window.location.origin}</code>
            </pre>
          </section>
        )}
      </div>
    </main>
  );
}
