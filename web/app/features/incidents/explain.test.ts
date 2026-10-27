import { describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import {
  IncidentLocation,
  IncidentSchema,
  SuspectSchema,
} from "~/gen/netwatch/v1/dashboard_pb";
import { explain, formatDuration } from "./explain";

const hostnames = { dad: "Dads-Laptop", me: "My-Mac" };

const incident = (fields: Partial<Parameters<typeof create>[1]> = {}) =>
  create(IncidentSchema, {
    start: timestampFromMs(0),
    end: timestampFromMs(23_000),
    location: IncidentLocation.UPSTREAM,
    baselineMs: 18.2,
    peakMs: 181,
    pings: 23,
    lostPings: 0,
    ...fields,
  });

describe("explain", () => {
  test("names the uploader and explains bufferbloat", () => {
    const e = explain(
      incident({
        suspects: [
          create(SuspectSchema, {
            agentId: "dad",
            peakUploadBytesPerSec: 1_175_000,
          }),
        ],
      }),
      hostnames,
      30_000,
    );
    expect(e.duration).toBe("23 s");
    expect(e.headline).toBe("Internet ping rose to 181 ms (normally 18 ms).");
    expect(e.cause).toStartWith(
      "At the same time, Dads-Laptop was uploading up to 9.4 Mbps.",
    );
    expect(e.cause).toContain("bufferbloat");
  });

  test("a big download, with another computer also busy", () => {
    const e = explain(
      incident({
        suspects: [
          create(SuspectSchema, {
            agentId: "me",
            peakDownloadBytesPerSec: 5_000_000,
          }),
          create(SuspectSchema, {
            agentId: "dad",
            peakDownloadBytesPerSec: 1_000_000,
          }),
        ],
      }),
      hostnames,
      30_000,
    );
    expect(e.cause).toBe(
      "At the same time, My-Mac was downloading up to 40.0 Mbps (also busy: Dads-Laptop, downloading up to 8.0 Mbps).",
    );
  });

  test("slow router means Wi-Fi", () => {
    const e = explain(
      incident({ location: IncidentLocation.LOCAL }),
      hostnames,
      30_000,
    );
    expect(e.cause).toContain("Wi-Fi");
  });

  test("nobody busy points at devices without an agent", () => {
    const e = explain(incident(), hostnames, 30_000);
    expect(e.cause).toContain("Xbox");
  });

  test("ongoing incidents run until now, and report loss", () => {
    const e = explain(
      incident({ end: undefined, lostPings: 5, pings: 20 }),
      hostnames,
      125_000,
    );
    expect(e.endMs).toBeNull();
    expect(e.duration).toBe("2 min 5 s");
    expect(e.headline).toContain("25% of pings lost");
  });
});

test("formatDuration", () => {
  expect(formatDuration(400)).toBe("1 s");
  expect(formatDuration(120_000)).toBe("2 min");
});
