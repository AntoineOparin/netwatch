import { expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { GetUptimeResponseSchema } from "~/gen/netwatch/v1/dashboard_pb";
import { summarizeUptime } from "./format";

const uptime = (monitored: number, downtime: number, outages = 0) =>
  create(GetUptimeResponseSchema, {
    monitored: { seconds: BigInt(monitored) },
    downtime: { seconds: BigInt(downtime) },
    outages,
  });

test("no outages is 100%", () => {
  expect(summarizeUptime(uptime(86_400, 0)).percent).toBe("100%");
});

test("any outage shows below 100%", () => {
  expect(summarizeUptime(uptime(86_400, 1, 1)).percent).toBe("99.99%");
  expect(summarizeUptime(uptime(86_400, 26, 2)).percent).toBe("99.96%");
});

test("nothing watched yet", () => {
  expect(summarizeUptime(uptime(0, 0)).percent).toBe("—");
});
