import { timestampMs } from "@bufbuild/protobuf/wkt";
import { useEffect, useState } from "react";
import {
  type LatencyPoint,
  LatencyTarget,
} from "~/gen/netwatch/v1/dashboard_pb";
import { dashboard } from "~/lib/api";
import type { LatencyData, Target } from "./stats";

const TARGET_NAMES: Partial<Record<LatencyTarget, Target>> = {
  [LatencyTarget.GATEWAY]: "gateway",
  [LatencyTarget.ISP]: "isp",
  [LatencyTarget.INTERNET]: "internet",
};

const RECONNECT_MS = 2000;

// Keeps a live window of probe results. The hub sends history first and then
// each new point; if the stream drops (hub restart), it reconnects and the
// history replaces whatever was kept.
export function useLatencyStream(windowMs: number) {
  const [data, setData] = useState<LatencyData>({});
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    const abort = new AbortController();

    (async () => {
      while (!abort.signal.aborted) {
        let current: LatencyData = {};
        try {
          const stream = dashboard().watchLatency(
            { history: { seconds: BigInt(Math.round(windowMs / 1000)) } },
            { signal: abort.signal },
          );
          let first = true;
          for await (const res of stream) {
            current = merge(first ? {} : current, res.points, windowMs);
            first = false;
            setData(current);
            setConnected(true);
          }
        } catch {
          // Aborted on unmount, or the hub went away: fall through to retry.
        }
        if (abort.signal.aborted) return;
        setConnected(false);
        await new Promise((r) => setTimeout(r, RECONNECT_MS));
      }
    })();

    return () => abort.abort();
  }, [windowMs]);

  return { data, connected };
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
