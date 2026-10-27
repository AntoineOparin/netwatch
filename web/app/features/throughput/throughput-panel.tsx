import { InfoTip, Term } from "~/components/info-tip";
import { SectionHeader } from "~/components/page-header";
import {
  type Highlight,
  TimeSeriesChart,
} from "~/components/time-series-chart";
import {
  describeWifi,
  WIFI_QUALITY_COLOR,
  wifiQuality,
} from "~/features/wifi/describe";
import type { AppUsage, WifiStatus } from "~/gen/netwatch/v1/dashboard_pb";
import { appsByAgent, describeApps } from "./apps";
import {
  type DeviceTraffic,
  deviceColor,
  formatRate,
  type RateSample,
  summarize,
  toMbps,
} from "./stats";
import { useThroughputStream } from "./use-throughput-stream";

const SUMMARY_WINDOW_MS = 60_000;

interface Props {
  windowMs: number;
  hostnames: Record<string, string>; // agent ID -> hostname
  agentOS: Record<string, string>; // agent ID -> "darwin", "windows", ...
  apps: AppUsage[]; // busiest apps over the last minute
  wifi: Record<string, WifiStatus | undefined>; // agent ID -> latest reading
  highlights: Highlight[];
}

export function ThroughputPanel({
  windowMs,
  hostnames,
  agentOS,
  apps,
  wifi,
  highlights,
}: Props) {
  const topApps = appsByAgent(apps);
  const { data } = useThroughputStream(windowMs);
  const ids = Object.keys(data).sort((a, b) =>
    (hostnames[a] ?? a).localeCompare(hostnames[b] ?? b),
  );
  const newest = Math.max(
    0,
    ...Object.values(data).flatMap((t) => [
      t.download.at(-1)?.at ?? 0,
      t.upload.at(-1)?.at ?? 0,
    ]),
  );
  const name = (id: string) => hostnames[id] ?? id;

  const chartSeries = (pick: (t: DeviceTraffic) => RateSample[]) =>
    ids.map((id) => ({
      key: id,
      label: name(id),
      color: deviceColor(id, ids),
      points: pick(data[id]).map((s) => ({
        at: s.at,
        value: toMbps(s.bytesPerSec),
      })),
    }));
  const formatMbps = (v: number) =>
    v >= 10 ? `${Math.round(v)} Mbps` : `${v.toFixed(1)} Mbps`;

  return (
    <section>
      <SectionHeader
        title="Who's using the internet"
        description={
          <>
            Computers running the <Term id="agent">agent</Term>. Watch the
            upload: a full upload queue delays everyone's packets, the Xbox's
            included (<Term id="bufferbloat">bufferbloat</Term>).
          </>
        }
      />

      {ids.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-zinc-300 p-6 dark:border-zinc-700">
          <p className="text-sm font-medium">No computers reporting yet</p>
          <p className="mt-1 max-w-xl text-sm text-zinc-500">
            Run the agent on a computer (
            <code className="font-mono text-xs">
              agent -hub {window.location.origin}
            </code>
            ) and its download and upload show up here within a few seconds.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          <div className="panel overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-zinc-950/[0.06] text-xs text-zinc-500 dark:border-white/[0.06]">
                <tr>
                  <th className="px-5 py-3 font-medium">Computer</th>
                  <th className="px-5 py-3 text-right font-medium">
                    <span className="inline-flex items-center gap-1">
                      Download now <InfoTip id="updown" />
                    </span>
                  </th>
                  <th className="px-5 py-3 text-right font-medium">
                    Upload now
                  </th>
                  <th className="px-5 py-3 text-right font-medium">
                    <span className="inline-flex items-center gap-1">
                      Upload, 1 min avg <InfoTip id="mbps" />
                    </span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-950/[0.05] dark:divide-white/[0.05]">
                {ids.map((id) => {
                  const s = summarize(id, data[id], newest, SUMMARY_WINDOW_MS);
                  return (
                    <tr key={id}>
                      <td className="px-5 py-3.5">
                        <div className="flex items-center gap-2 font-medium">
                          <span
                            className="h-0.5 w-4 shrink-0 rounded-full"
                            style={{ background: deviceColor(id, ids) }}
                          />
                          {name(id)}
                          <span className="font-mono text-xs font-normal text-zinc-400">
                            {s.iface}
                          </span>
                        </div>
                        <WifiLine wifi={wifi[id]} />
                        <AppLine apps={topApps[id] ?? []} os={agentOS[id]} />
                      </td>
                      <td className="px-5 py-3.5 text-right font-medium tabular-nums">
                        {formatRate(s.downNow)}
                      </td>
                      <td className="px-5 py-3.5 text-right font-medium tabular-nums">
                        {formatRate(s.upNow)}
                      </td>
                      <td className="px-5 py-3.5 text-right text-zinc-600 tabular-nums dark:text-zinc-400">
                        {formatRate(s.upAvg)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            {(
              [
                ["Download", (t: DeviceTraffic) => t.download],
                ["Upload", (t: DeviceTraffic) => t.upload],
              ] as const
            ).map(([title, pick]) => (
              <div key={title} className="panel p-5">
                <h3 className="mb-1 text-sm font-semibold">{title}</h3>
                <ul className="mb-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-600 dark:text-zinc-400">
                  {ids.map((id) => (
                    <li key={id} className="flex items-center gap-1.5">
                      <span
                        className="h-0.5 w-4 rounded-full"
                        style={{ background: deviceColor(id, ids) }}
                      />
                      {name(id)}
                    </li>
                  ))}
                </ul>
                <TimeSeriesChart
                  series={chartSeries(pick)}
                  windowMs={windowMs}
                  formatValue={formatMbps}
                  yFloor={1}
                  height={200}
                  highlights={highlights}
                />
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  );
}

// Which apps were busy on this computer in the last minute.
function AppLine({ apps, os }: { apps: AppUsage[]; os?: string }) {
  let content: React.ReactNode;
  if (apps.length) {
    content = describeApps(apps).map((a, i) => (
      <span key={a.app}>
        {i > 0 && " · "}
        <span className="text-zinc-700 dark:text-zinc-300">{a.app}</span>{" "}
        <span className="tabular-nums">{a.text}</span>
      </span>
    ));
  } else if (os && os !== "darwin") {
    content = "Per-app breakdown is macOS-only for now.";
  } else {
    content = "No app was busy in the last minute.";
  }
  return <p className="mt-1 pl-6 text-xs text-zinc-500">{content}</p>;
}

// This computer's Wi-Fi signal, if it's on Wi-Fi.
function WifiLine({ wifi }: { wifi?: WifiStatus }) {
  if (!wifi) return null;
  const quality = wifiQuality(wifi.signalDbm);
  return (
    <p className="mt-1 flex items-center gap-1.5 pl-6 text-xs text-zinc-500">
      <span
        aria-hidden
        className="size-1.5 rounded-full"
        style={{ background: WIFI_QUALITY_COLOR[quality] }}
      />
      <span>
        Wi-Fi signal{" "}
        <span className="font-medium text-zinc-700 dark:text-zinc-300">
          {quality}
        </span>{" "}
        · {describeWifi(wifi)}
      </span>
      <InfoTip id="wifiSignal" />
    </p>
  );
}
