import { timestampDate, timestampMs } from "@bufbuild/protobuf/wkt";
import { useEffect, useState } from "react";
import { Term } from "~/components/info-tip";
import {
  type ChartSeries,
  TimeSeriesChart,
} from "~/components/time-series-chart";
import { type WindowOption, WindowPicker } from "~/components/window-picker";
import { computeStats } from "~/features/latency/stats";
import type { LanDevice } from "~/gen/netwatch/v1/dashboard_pb";
import { dashboard, timeAgo } from "~/lib/api";
import { displayName } from "./describe";

const MIN = 60_000;
const WINDOWS: WindowOption[] = [
  { label: "5 min", ms: 5 * MIN },
  { label: "15 min", ms: 15 * MIN },
  { label: "1 hour", ms: 60 * MIN },
  { label: "6 hours", ms: 6 * 60 * MIN },
  { label: "24 hours", ms: 24 * 60 * MIN },
];
const REFRESH_MS = 10_000;

type Loaded = (ChartSeries & { device: LanDevice })[];

// One device's LAN ping next to the router's. The router is the baseline:
// if the device's line sits well above it, the device's own Wi-Fi link is
// adding the delay.
export function DevicePingChart({
  device,
  router,
}: {
  device: LanDevice;
  router?: LanDevice;
}) {
  const [windowMs, setWindowMs] = useState(WINDOWS[1].ms);
  const [series, setSeries] = useState<Loaded | null>(null);
  const [failed, setFailed] = useState(false);

  // biome-ignore lint/correctness/useExhaustiveDependencies: refetch when the devices or window change, not on every poll of their stats.
  useEffect(() => {
    const shown =
      router && router.mac !== device.mac ? [device, router] : [device];
    let cancelled = false;
    const load = async () => {
      try {
        const results = await Promise.all(
          shown.map((d) =>
            dashboard().getDevicePing({
              mac: d.mac,
              window: { seconds: BigInt(windowMs / 1000) },
            }),
          ),
        );
        if (cancelled) return;
        setFailed(false);
        setSeries(
          shown.map((d, i) => ({
            device: d,
            key: d.mac,
            label: displayName(d),
            color: `var(--series-${i + 1})`,
            points: results[i].points.flatMap((p) =>
              p.at
                ? [{ at: timestampMs(p.at), value: p.lost ? null : p.rttMs }]
                : [],
            ),
          })),
        );
      } catch {
        if (!cancelled) setFailed(true);
      }
    };
    load();
    const id = setInterval(load, REFRESH_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [device.mac, router?.mac, windowMs]);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm font-medium">
          <Term id="lanPing">LAN ping</Term> for {displayName(device)}
        </p>
        <WindowPicker
          value={windowMs}
          onChange={(ms) => {
            setSeries(null);
            setWindowMs(ms);
          }}
          options={WINDOWS}
        />
      </div>

      {failed && !series ? (
        <p className="text-sm text-zinc-500">Couldn't load the ping history.</p>
      ) : !series ? (
        <div className="h-[260px] animate-pulse rounded-xl bg-zinc-100 motion-reduce:animate-none dark:bg-zinc-800/60" />
      ) : (
        <>
          {series[0].points.length === 0 && (
            <p className="rounded-lg bg-zinc-100 px-3 py-2 text-sm text-zinc-600 dark:bg-zinc-800/60 dark:text-zinc-300">
              No pings from {displayName(device)} in this window
              {device.lastSeen
                ? `; it was last seen ${timeAgo(timestampDate(device.lastSeen))}`
                : ""}
              . Try a longer window.
            </p>
          )}
          <Stats series={series} />
          <TimeSeriesChart
            series={series}
            windowMs={windowMs}
            formatValue={(v) => `${Math.round(v)} ms`}
            yFloor={20}
            markMissing
            height={260}
            zoomable
          />
          <p className="max-w-2xl text-xs text-zinc-500">
            Drag across the chart to zoom in; double-click to zoom out. Red
            ticks are lost pings.
            {series.length > 1 &&
              ` If ${displayName(device)}'s line sits well above your router's, its own Wi-Fi link is adding the delay: try moving it closer to the router, or use a cable.`}
          </p>
        </>
      )}
    </div>
  );
}

const fmt = (v: number | null) => (v === null ? "—" : `${Math.round(v)} ms`);

// Average, jitter, peak and loss over the selected window, per line.
function Stats({ series }: { series: Loaded }) {
  return (
    <table className="w-full max-w-xl text-left text-xs">
      <thead className="text-zinc-500">
        <tr>
          <th className="py-1 pr-4 font-medium" />
          <th className="py-1 pr-4 text-right font-medium">Average</th>
          <th className="py-1 pr-4 text-right font-medium">
            <Term id="jitter">Jitter</Term>
          </th>
          <th className="py-1 pr-4 text-right font-medium">Peak</th>
          <th className="py-1 text-right font-medium">
            <Term id="loss">Lost</Term>
          </th>
        </tr>
      </thead>
      <tbody>
        {series.map((s) => {
          const samples = s.points.map((p) => ({ at: p.at, rtt: p.value }));
          const st = computeStats({ addr: "", samples }, 0);
          const peak = Math.max(0, ...samples.flatMap((p) => p.rtt ?? []));
          return (
            <tr key={s.key}>
              <th scope="row" className="py-1 pr-4 font-medium">
                <span className="flex items-center gap-1.5">
                  <span
                    aria-hidden
                    className="h-0.5 w-4 rounded-full"
                    style={{ background: s.color }}
                  />
                  {s.label}
                </span>
              </th>
              <td className="py-1 pr-4 text-right tabular-nums">
                {fmt(st.avg)}
              </td>
              <td className="py-1 pr-4 text-right tabular-nums">
                {fmt(st.jitter)}
              </td>
              <td className="py-1 pr-4 text-right tabular-nums">
                {st.count ? fmt(peak) : "—"}
              </td>
              <td className="py-1 text-right tabular-nums">
                {st.count ? `${Math.round(st.loss * 100)}%` : "—"}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
