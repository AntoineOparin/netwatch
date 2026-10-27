// One line per computer saying which apps are busy.

import type { AppUsage } from "~/gen/netwatch/v1/dashboard_pb";
import { formatRate } from "./stats";

const SHOWN = 3;

// Each app with the direction it mostly used: "OneDrive ↑ 8.0 Mbps".
export function describeApps(apps: AppUsage[]) {
  return apps.slice(0, SHOWN).map((a) => {
    const up = a.avgUploadBytesPerSec > a.avgDownloadBytesPerSec;
    const rate = up ? a.avgUploadBytesPerSec : a.avgDownloadBytesPerSec;
    return {
      app: a.app,
      direction: up ? "up" : "down",
      text: `${up ? "↑" : "↓"} ${formatRate(rate)}`,
    } as const;
  });
}

export function appsByAgent(apps: AppUsage[]) {
  const out: Record<string, AppUsage[]> = {};
  for (const a of apps) {
    out[a.agentId] ??= [];
    out[a.agentId].push(a);
  }
  return out;
}
