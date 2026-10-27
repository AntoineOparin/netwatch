import { expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { WifiStatusSchema } from "~/gen/netwatch/v1/dashboard_pb";
import { describeWifi, wifiQuality } from "./describe";

test("signal bands", () => {
  expect(wifiQuality(-48)).toBe("excellent");
  expect(wifiQuality(-62)).toBe("good");
  expect(wifiQuality(-68)).toBe("fair");
  expect(wifiQuality(-81)).toBe("poor");
});

test("describeWifi", () => {
  expect(
    describeWifi(
      create(WifiStatusSchema, { signalDbm: -68, bandGhz: 5, txRateMbps: 520 }),
    ),
  ).toBe("5 GHz · −68 dBm · 520 Mbps link");
  expect(describeWifi(create(WifiStatusSchema, { signalDbm: -56 }))).toBe(
    "−56 dBm",
  );
});
