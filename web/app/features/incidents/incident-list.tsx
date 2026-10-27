import type { Incident } from "~/gen/netwatch/v1/dashboard_pb";
import { explain } from "./explain";

interface Props {
  incidents: Incident[];
  hostnames: Record<string, string>;
  windowMs: number;
}

// Incidents overlapping the selected window, newest first.
export function IncidentList({ incidents, hostnames, windowMs }: Props) {
  const now = Date.now();
  const shown = incidents
    .map((inc) => ({ id: inc.id, ...explain(inc, hostnames, now) }))
    .filter((e) => (e.endMs ?? now) >= now - windowMs);

  return (
    <section className="space-y-3">
      <div>
        <h2 className="text-lg font-semibold">Lag incidents</h2>
        <p className="text-sm text-gray-500">
          When internet pings got much slower than normal, and what was busy at
          the same time. Busy at the same moment is a strong hint, not proof.
        </p>
      </div>

      {shown.length === 0 ? (
        <p className="rounded-xl border border-dashed border-gray-300 p-5 text-sm text-gray-500 dark:border-gray-700">
          No lag in this window.
        </p>
      ) : (
        <ol className="space-y-2">
          {shown.map((e) => {
            const ongoing = e.endMs === null;
            return (
              <li
                key={String(e.id)}
                className="flex gap-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900"
              >
                <span
                  aria-hidden
                  className="flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-bold text-white"
                  style={{
                    background: ongoing
                      ? "var(--status-critical)"
                      : "var(--status-warning)",
                  }}
                >
                  !
                </span>
                <div className="min-w-0 space-y-0.5 text-sm">
                  <p className="flex flex-wrap items-baseline gap-x-2">
                    <span className="font-semibold">
                      {ongoing
                        ? "Happening now"
                        : new Date(e.startMs).toLocaleTimeString()}
                    </span>
                    <span className="text-gray-500">{e.duration}</span>
                  </p>
                  <p>{e.headline}</p>
                  <p className="text-gray-600 dark:text-gray-400">{e.cause}</p>
                </div>
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}

// Time ranges to shade on charts.
export function incidentRanges(incidents: Incident[], nowMs: number) {
  return incidents.map((inc) => {
    const e = explain(inc, {}, nowMs);
    return { from: e.startMs, to: e.endMs ?? nowMs };
  });
}
