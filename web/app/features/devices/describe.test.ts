import { describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { LanDeviceSchema } from "~/gen/netwatch/v1/dashboard_pb";
import { details, displayName, pingSummary } from "./describe";

const device = (fields: Parameters<typeof create<typeof LanDeviceSchema>>[1]) =>
  create(LanDeviceSchema, fields);

describe("displayName", () => {
  test("nickname, then router, then hostname, then maker", () => {
    expect(displayName(device({ nickname: "Xbox", hostname: "XboxOne" }))).toBe(
      "Xbox",
    );
    expect(
      displayName(device({ isRouter: true, hostname: "puma7-atom" })),
    ).toBe("Your router");
    expect(displayName(device({ hostname: "iPad" }))).toBe("iPad");
    expect(displayName(device({ vendor: "Intel Corporate" }))).toBe(
      "Intel Corporate",
    );
    expect(displayName(device({}))).toBe("Unknown device");
  });
});

test("details skips what the name already says", () => {
  expect(details(device({ hostname: "G6", vendor: "LG Electronics" }))).toBe(
    "LG Electronics",
  );
  expect(
    details(
      device({ nickname: "Mum's phone", hostname: "iPhone", privateMac: true }),
    ),
  ).toBe("iPhone · private address, maker hidden");
});

describe("pingSummary", () => {
  const pinging = { answersPing: true, pings: 120, pingJitterMs: 1 };

  test("fast and steady is good", () => {
    expect(pingSummary(device({ ...pinging, pingAvgMs: 3.2 })).quality).toBe(
      "good",
    );
  });

  test("a dozing phone is slow, and says why", () => {
    const s = pingSummary(device({ ...pinging, pingAvgMs: 117 }));
    expect(s.quality).toBe("slow");
    expect(s.label).toBe("117 ms");
    expect(s.detail).toContain("saving battery");
  });

  test("loss alone makes it fair or slow", () => {
    expect(
      pingSummary(device({ ...pinging, pingAvgMs: 4, pingLoss: 0.1 })).quality,
    ).toBe("fair");
    expect(
      pingSummary(device({ ...pinging, pingAvgMs: 4, pingLoss: 0.3 })).quality,
    ).toBe("slow");
  });

  test("devices that ignore ping", () => {
    expect(pingSummary(device({})).label).toBe("No reply");
  });
});
