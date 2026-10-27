import { timestampMs } from "@bufbuild/protobuf/wkt";
import { useEffect, useState } from "react";
import {
  type ChartSeries,
  TimeSeriesChart,
} from "~/components/time-series-chart";
import type { LanDevice } from "~/gen/netwatch/v1/dashboard_pb";
import { dashboard } from "~/lib/api";
import { displayName } from "./describe";

const WINDOW_MS = 60 * 60_000;
const REFRESH_MS = 10_000;

// One device's LAN ping over the last hour, next to the router's. The router
// is the baseline: if the device's line sits well above it, the device's own
// Wi-Fi link is adding the delay.
export function DevicePingChart({
  device,
  router,
}: {
  device: LanDevice;
  router?: LanDevice;
}) {
  const [series, setSeries] = useState<ChartSeries[] | null>(null);
  const [failed, setFailed] = useState(false);

  // biome-ignore lint/correctness/useExhaustiveDependencies: refetch only when the devices change, not on every poll of their stats.
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
              window: { seconds: BigInt(WINDOW_MS / 1000) },
            }),
          ),
        );
        if (cancelled) return;
        setFailed(false);
        setSeries(
          shown.map((d, i) => ({
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
  }, [device.mac, router?.mac]);

  if (failed && !series) {
    return (
      <p className="text-sm text-zinc-500">Couldn't load the ping history.</p>
    );
  }
  if (!series) {
    return (
      <div className="h-[200px] animate-pulse rounded-xl bg-zinc-100 motion-reduce:animate-none dark:bg-zinc-800/60" />
    );
  }
  return (
    <div>
      <div className="mb-2 flex flex-wrap items-baseline justify-between gap-2">
        <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-600 dark:text-zinc-400">
          {series.map((s) => (
            <li key={s.key} className="flex items-center gap-1.5">
              <span
                aria-hidden
                className="h-0.5 w-4 rounded-full"
                style={{ background: s.color }}
              />
              {s.label}
            </li>
          ))}
        </ul>
        <p className="text-xs text-zinc-500">
          Last hour · red ticks are lost pings
        </p>
      </div>
      <TimeSeriesChart
        series={series}
        windowMs={WINDOW_MS}
        formatValue={(v) => `${Math.round(v)} ms`}
        yFloor={20}
        markMissing
        height={200}
      />
      {series.length > 1 && (
        <p className="mt-2 max-w-2xl text-xs text-zinc-500">
          If {displayName(device)}'s line sits well above your router's, its own
          Wi-Fi link is adding the delay: try moving it closer to the router, or
          use a cable.
        </p>
      )}
    </div>
  );
}
