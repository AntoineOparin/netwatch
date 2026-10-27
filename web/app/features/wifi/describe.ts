// How strong a Wi-Fi signal is, in words. dBm is negative: closer to zero is
// stronger. These bands are the usual rules of thumb (-67 dBm is the common
// minimum for smooth calls and gaming).

import type { WifiStatus } from "~/gen/netwatch/v1/dashboard_pb";

export type WifiQuality = "excellent" | "good" | "fair" | "poor";

export function wifiQuality(signalDbm: number): WifiQuality {
  if (signalDbm >= -55) return "excellent";
  if (signalDbm >= -67) return "good";
  if (signalDbm >= -75) return "fair";
  return "poor";
}

export const WIFI_QUALITY_COLOR: Record<WifiQuality, string> = {
  excellent: "var(--status-good)",
  good: "var(--status-good)",
  fair: "var(--status-warning)",
  poor: "var(--status-critical)",
};

// e.g. "5 GHz · −68 dBm · 520 Mbps link"
export function describeWifi(w: WifiStatus) {
  const parts: string[] = [];
  if (w.bandGhz) parts.push(`${w.bandGhz} GHz`);
  parts.push(`${Math.round(w.signalDbm)} dBm`.replace("-", "−"));
  if (w.txRateMbps) parts.push(`${Math.round(w.txRateMbps)} Mbps link`);
  return parts.join(" · ");
}
