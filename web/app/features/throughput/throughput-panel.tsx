import {
  type Highlight,
  TimeSeriesChart,
} from "~/components/time-series-chart";
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
  highlights: Highlight[];
}

export function ThroughputPanel({ windowMs, hostnames, highlights }: Props) {
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
    <section className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold">Who's using the internet</h2>
        <p className="text-sm text-gray-500">
          Computers running the agent. Watch the upload: a full upload queue
          delays everyone's packets, the Xbox's included.
        </p>
      </div>

      {ids.length === 0 ? (
        <p className="rounded-xl border border-dashed border-gray-300 p-5 text-sm text-gray-500 dark:border-gray-700">
          No throughput yet. Start the agent on a computer and its traffic shows
          up here within a few seconds.
        </p>
      ) : (
        <>
          <div className="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-gray-200 text-gray-500 dark:border-gray-800">
                <tr>
                  <th className="px-4 py-2 font-medium">Computer</th>
                  <th className="px-4 py-2 text-right font-medium">
                    Download now
                  </th>
                  <th className="px-4 py-2 text-right font-medium">
                    Upload now
                  </th>
                  <th className="px-4 py-2 text-right font-medium">
                    Upload, 1 min avg
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
                {ids.map((id) => {
                  const s = summarize(id, data[id], newest, SUMMARY_WINDOW_MS);
                  return (
                    <tr key={id}>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2 font-medium">
                          <span
                            className="h-0.5 w-4 shrink-0 rounded-full"
                            style={{ background: deviceColor(id, ids) }}
                          />
                          {name(id)}
                          <span className="font-mono text-xs font-normal text-gray-400">
                            {s.iface}
                          </span>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums">
                        {formatRate(s.downNow)}
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums">
                        {formatRate(s.upNow)}
                      </td>
                      <td className="px-4 py-3 text-right text-gray-600 tabular-nums dark:text-gray-400">
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
              <div
                key={title}
                className="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900"
              >
                <h3 className="mb-1 text-sm font-medium">{title}</h3>
                <ul className="mb-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-600 dark:text-gray-400">
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
        </>
      )}
    </section>
  );
}
