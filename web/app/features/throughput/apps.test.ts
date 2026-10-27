import { expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { AppUsageSchema } from "~/gen/netwatch/v1/dashboard_pb";
import { appsByAgent, describeApps } from "./apps";

test("each app shows the direction it mostly used", () => {
  const apps = [
    create(AppUsageSchema, {
      agentId: "a",
      app: "OneDrive",
      avgUploadBytesPerSec: 1_000_000,
    }),
    create(AppUsageSchema, {
      agentId: "a",
      app: "Google Chrome",
      avgDownloadBytesPerSec: 500_000,
      avgUploadBytesPerSec: 10_000,
    }),
    create(AppUsageSchema, {
      agentId: "b",
      app: "Spotify",
      avgDownloadBytesPerSec: 40_000,
    }),
  ];
  const grouped = appsByAgent(apps);
  expect(Object.keys(grouped)).toEqual(["a", "b"]);
  expect(describeApps(grouped.a)).toEqual([
    { app: "OneDrive", direction: "up", text: "↑ 8.0 Mbps" },
    { app: "Google Chrome", direction: "down", text: "↓ 4.0 Mbps" },
  ]);
});
