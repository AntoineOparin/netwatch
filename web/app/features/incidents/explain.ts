// Turns an incident into the plain-language sentences the dashboard shows.

import { timestampMs } from "@bufbuild/protobuf/wkt";
import { formatRate } from "~/features/throughput/stats";
import {
  type Incident,
  IncidentLocation,
} from "~/gen/netwatch/v1/dashboard_pb";

export interface Explanation {
  startMs: number;
  endMs: number | null; // null while ongoing
  duration: string;
  headline: string;
  cause: string;
}

const ONE_MBPS = 125_000; // bytes/sec

export function formatDuration(ms: number) {
  const s = Math.max(1, Math.round(ms / 1000));
  if (s < 60) return `${s} s`;
  const m = Math.floor(s / 60);
  return s % 60 ? `${m} min ${s % 60} s` : `${m} min`;
}

export function explain(
  inc: Incident,
  hostnames: Record<string, string>,
  nowMs: number,
): Explanation {
  const startMs = inc.start ? timestampMs(inc.start) : nowMs;
  const endMs = inc.end ? timestampMs(inc.end) : null;
  const duration = formatDuration((endMs ?? nowMs) - startMs);
  const lostPct = inc.pings ? Math.round((inc.lostPings / inc.pings) * 100) : 0;

  const parts = [
    inc.peakMs > 0
      ? `Internet ping rose to ${Math.round(inc.peakMs)} ms (normally ${Math.round(inc.baselineMs)} ms)`
      : "Internet pings were being lost",
  ];
  if (inc.peakMs > 0 && lostPct > 0) parts.push(`${lostPct}% of pings lost`);
  const headline = `${parts.join(", ")}.`;

  return { startMs, endMs, duration, headline, cause: cause(inc, hostnames) };
}

function cause(inc: Incident, hostnames: Record<string, string>) {
  if (inc.location === IncidentLocation.LOCAL) {
    return "Your router was slow to answer too, so this was Wi-Fi or the local network, not someone using bandwidth.";
  }
  const [top, ...others] = inc.suspects;
  if (!top) {
    return "None of the computers running the agent were busy. Likely a device without one (the Xbox itself, a phone, a TV) or the ISP.";
  }
  const name = (id: string) => hostnames[id] ?? id;
  const activity = (s: (typeof inc.suspects)[number]) =>
    s.peakUploadBytesPerSec >= ONE_MBPS
      ? `uploading up to ${formatRate(s.peakUploadBytesPerSec)}`
      : `downloading up to ${formatRate(s.peakDownloadBytesPerSec)}`;

  let text = `At the same time, ${name(top.agentId)} was ${activity(top)}`;
  if (others.length) {
    text += ` (also busy: ${others.map((s) => `${name(s.agentId)}, ${activity(s)}`).join("; ")})`;
  }
  text += ".";
  if (top.peakUploadBytesPerSec >= ONE_MBPS) {
    text +=
      " Uploads fill the modem's queue and delay everyone's packets (bufferbloat).";
  }
  return text;
}
