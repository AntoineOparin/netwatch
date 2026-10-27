import { Link } from "react-router";
import { ArrowRightIcon } from "~/components/icons";
import { InfoTip } from "~/components/info-tip";
import { SectionHeader } from "~/components/page-header";
import type { Incident } from "~/gen/netwatch/v1/dashboard_pb";
import { type Explanation, explain } from "./explain";

type Explained = Explanation & { id: bigint; outage: boolean };

// Incidents overlapping the last windowMs, newest first.
function explainAll(
  incidents: Incident[],
  hostnames: Record<string, string>,
  windowMs: number,
): Explained[] {
  const now = Date.now();
  return incidents
    .map((inc) => ({
      id: inc.id,
      outage: inc.outage,
      ...explain(inc, hostnames, now),
    }))
    .filter((e) => (e.endMs ?? now) >= now - windowMs);
}

function IncidentItem({
  e,
  compact,
  className = "",
}: {
  e: Explained;
  compact?: boolean;
  className?: string;
}) {
  const ongoing = e.endMs === null;
  const when = new Date(e.startMs);
  return (
    <li className={`flex gap-3 ${className}`}>
      <span
        aria-hidden
        className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-lg text-xs font-bold text-white"
        style={{
          background:
            ongoing || e.outage
              ? "var(--status-critical)"
              : "var(--status-warning)",
        }}
      >
        {e.outage ? "✕" : "!"}
      </span>
      <div className="min-w-0 text-sm">
        <p className="flex flex-wrap items-baseline gap-x-2">
          <span className="font-semibold">
            {ongoing
              ? "Happening now"
              : when.toLocaleTimeString([], {
                  hour: "numeric",
                  minute: "2-digit",
                  second: "2-digit",
                })}
          </span>
          {!compact && !ongoing && (
            <span className="text-zinc-500">{when.toLocaleDateString()}</span>
          )}
          <span className="text-zinc-500">· {e.duration}</span>
          {e.outage && (
            <span className="rounded bg-zinc-900 px-1.5 py-0.5 text-[11px] font-medium text-white dark:bg-zinc-100 dark:text-zinc-900">
              Outage
            </span>
          )}
        </p>
        <p className="mt-0.5">{e.headline}</p>
        <p
          className={`mt-0.5 text-zinc-600 dark:text-zinc-400 ${compact ? "line-clamp-3" : ""}`}
        >
          {e.cause}
        </p>
      </div>
    </li>
  );
}

const RECENT = 3;

// The overview's side column: the latest few incidents.
export function RecentIncidents({
  incidents,
  hostnames,
  windowMs,
}: {
  incidents: Incident[];
  hostnames: Record<string, string>;
  windowMs: number;
}) {
  const shown = explainAll(incidents, hostnames, windowMs);
  return (
    <section className="panel flex flex-col p-5">
      <SectionHeader
        title={
          <span className="inline-flex items-center gap-1">
            Recent lag <InfoTip id="incident" />
          </span>
        }
      />
      {shown.length === 0 ? (
        <div className="flex flex-1 flex-col items-start justify-center py-6">
          <p className="text-sm font-medium">No lag in this window</p>
          <p className="mt-1 text-sm text-zinc-500">
            When internet ping jumps well above normal, it shows up here with
            what was busy at the time.
          </p>
        </div>
      ) : (
        <ol className="space-y-4">
          {shown.slice(0, RECENT).map((e) => (
            <IncidentItem key={String(e.id)} e={e} compact />
          ))}
        </ol>
      )}
      <Link
        to="/incidents"
        className="mt-auto inline-flex items-center gap-1 pt-5 text-sm font-medium text-[var(--accent)] hover:underline"
      >
        {shown.length > RECENT
          ? `All ${shown.length} incidents`
          : "All incidents"}
        <ArrowRightIcon className="size-4" />
      </Link>
    </section>
  );
}

// The full list on the incidents page.
export function IncidentList({
  incidents,
  hostnames,
  windowMs,
}: {
  incidents: Incident[];
  hostnames: Record<string, string>;
  windowMs: number;
}) {
  const shown = explainAll(incidents, hostnames, windowMs);
  if (shown.length === 0) {
    return (
      <div className="panel p-8">
        <p className="font-medium">No lag in this window</p>
        <p className="mt-1 max-w-xl text-sm text-zinc-500">
          NetWatch flags a lag incident when internet ping rises 30 ms or more
          above its normal level, or pings start getting lost. Try a longer
          window, or come back after a laggy evening.
        </p>
      </div>
    );
  }
  return (
    <ol className="panel divide-y divide-zinc-950/[0.06] dark:divide-white/[0.06]">
      {shown.map((e) => (
        <IncidentItem key={String(e.id)} e={e} className="p-5" />
      ))}
    </ol>
  );
}

// Time ranges to shade on charts.
export function incidentRanges(incidents: Incident[], nowMs: number) {
  return incidents.map((inc) => {
    const e = explain(inc, {}, nowMs);
    return { from: e.startMs, to: e.endMs ?? nowMs };
  });
}
