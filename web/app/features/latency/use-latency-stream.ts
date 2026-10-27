import { timestampMs } from "@bufbuild/protobuf/wkt";
import {
  type LatencyPoint,
  LatencyTarget,
} from "~/gen/netwatch/v1/dashboard_pb";
import { dashboard } from "~/lib/api";
import { useLiveStream } from "~/lib/use-live-stream";
import type { LatencyData, Target } from "./stats";

const TARGET_NAMES: Partial<Record<LatencyTarget, Target>> = {
  [LatencyTarget.GATEWAY]: "gateway",
  [LatencyTarget.ISP]: "isp",
  [LatencyTarget.INTERNET]: "internet",
};

// Keeps a live window of probe results.
export function useLatencyStream(windowMs: number) {
  return useLiveStream<LatencyData, LatencyPoint>(
    (signal) =>
      dashboard().watchLatency(
        { history: { seconds: BigInt(Math.round(windowMs / 1000)) } },
        { signal },
      ),
    (state, points) => merge(state ?? {}, points, windowMs),
    {},
    windowMs,
  );
}

function merge(
  prev: LatencyData,
  points: LatencyPoint[],
  windowMs: number,
): LatencyData {
  const next: LatencyData = { ...prev };
  let newest = 0;
  for (const p of points) {
    const target = TARGET_NAMES[p.target];
    if (!target || !p.at) continue;
    const at = timestampMs(p.at);
    newest = Math.max(newest, at);
    const series = next[target];
    const sample = { at, rtt: p.lost ? null : p.rttMs };
    next[target] = {
      addr: p.addr,
      samples: series ? [...series.samples, sample] : [sample],
    };
  }
  const cutoff = newest - windowMs;
  for (const [target, series] of Object.entries(next)) {
    next[target as Target] = {
      ...series,
      samples: series.samples.filter((s) => s.at >= cutoff),
    };
  }
  return next;
}
