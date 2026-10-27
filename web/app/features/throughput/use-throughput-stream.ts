import { timestampMs } from "@bufbuild/protobuf/wkt";
import {
  Direction,
  type ThroughputPoint,
} from "~/gen/netwatch/v1/dashboard_pb";
import { dashboard } from "~/lib/api";
import { useLiveStream } from "~/lib/use-live-stream";
import type { ThroughputData } from "./stats";

// Keeps a live window of every agent's download and upload rates.
export function useThroughputStream(windowMs: number) {
  return useLiveStream<ThroughputData, ThroughputPoint>(
    (signal) =>
      dashboard().watchThroughput(
        { history: { seconds: BigInt(Math.round(windowMs / 1000)) } },
        { signal },
      ),
    (state, points) => merge(state ?? {}, points, windowMs),
    {},
    windowMs,
  );
}

function merge(
  prev: ThroughputData,
  points: ThroughputPoint[],
  windowMs: number,
): ThroughputData {
  const next: ThroughputData = { ...prev };
  let newest = 0;
  for (const p of points) {
    if (!p.at) continue;
    const at = timestampMs(p.at);
    newest = Math.max(newest, at);
    const cur = next[p.agentId] ?? {
      iface: p.interface,
      download: [],
      upload: [],
    };
    const sample = { at, bytesPerSec: p.bytesPerSec };
    next[p.agentId] =
      p.direction === Direction.UPLOAD
        ? { ...cur, iface: p.interface, upload: [...cur.upload, sample] }
        : { ...cur, iface: p.interface, download: [...cur.download, sample] };
  }
  const cutoff = newest - windowMs;
  for (const [id, t] of Object.entries(next)) {
    next[id] = {
      ...t,
      download: t.download.filter((s) => s.at >= cutoff),
      upload: t.upload.filter((s) => s.at >= cutoff),
    };
  }
  return next;
}
