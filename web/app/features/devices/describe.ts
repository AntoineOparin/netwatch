// How a LAN device is shown: its name, a line about what it is, and how
// healthy its connection looks.

import type { LanDevice } from "~/gen/netwatch/v1/dashboard_pb";

export function displayName(d: LanDevice) {
  if (d.nickname) return d.nickname;
  if (d.isRouter) return "Your router";
  return d.hostname || d.vendor || "Unknown device";
}

// The secondary line under the name: whatever the name doesn't already say.
export function details(d: LanDevice) {
  const parts: string[] = [];
  const name = displayName(d);
  if (d.hostname && d.hostname !== name) parts.push(d.hostname);
  if (d.vendor && d.vendor !== name) parts.push(d.vendor);
  if (d.privateMac) parts.push("private address, maker hidden");
  return parts.join(" · ");
}

export type Quality = "good" | "fair" | "slow" | "none";

export interface PingSummary {
  quality: Quality;
  label: string;
  detail: string;
}

// A healthy device on Wi-Fi answers in a few milliseconds. Much slower
// usually means a weak signal, or a phone dozing to save battery (it only
// wakes for Wi-Fi every ~100 ms or more).
export function pingSummary(d: LanDevice): PingSummary {
  if (d.isHub) return { quality: "none", label: "—", detail: "this machine" };
  if (!d.answersPing) {
    return {
      quality: "none",
      label: "No reply",
      detail: "doesn't answer ping (common for phones and consoles)",
    };
  }
  if (!d.pings) {
    return { quality: "none", label: "—", detail: "no recent pings" };
  }
  const avg = Math.round(d.pingAvgMs);
  const loss = Math.round(d.pingLoss * 100);
  const detail = `±${Math.round(d.pingJitterMs)} ms jitter${loss ? `, ${loss}% lost` : ""}`;
  if (avg < 15 && loss < 5) {
    return { quality: "good", label: `${avg} ms`, detail };
  }
  if (avg < 60 && loss < 15) {
    return { quality: "fair", label: `${avg} ms`, detail };
  }
  return {
    quality: "slow",
    label: `${avg} ms`,
    detail: `${detail}. Weak Wi-Fi, or a phone saving battery`,
  };
}
