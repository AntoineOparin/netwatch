import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useEffect, useRef, useState } from "react";
import { useFetcher } from "react-router";
import { InfoTip, Term } from "~/components/info-tip";
import { PageHeader, SectionHeader } from "~/components/page-header";
import {
  details,
  displayName,
  pingSummary,
  type Quality,
} from "~/features/devices/describe";
import { DevicePingChart } from "~/features/devices/ping-chart";
import {
  describeWifi,
  WIFI_QUALITY_COLOR,
  wifiQuality,
} from "~/features/wifi/describe";
import type { LanDevice, WifiStatus } from "~/gen/netwatch/v1/dashboard_pb";
import { dashboard, timeAgo, usePolling } from "~/lib/api";
import type { Route } from "./+types/devices";

export function meta() {
  return [{ title: "Devices · NetWatch" }];
}

export async function clientLoader() {
  const [{ devices }, { agents }] = await Promise.all([
    dashboard().listDevices({}),
    dashboard().listAgents({}),
  ]);
  return { devices, agents };
}

export async function clientAction({ request }: Route.ClientActionArgs) {
  const form = await request.formData();
  await dashboard().setDeviceNickname({
    mac: String(form.get("mac")),
    nickname: String(form.get("nickname") ?? ""),
  });
  return { ok: true };
}

export default function Devices({ loaderData }: Route.ComponentProps) {
  usePolling(10_000);
  const { devices, agents } = loaderData;
  const online = devices.filter((d) => d.online).length;
  const agentNames = Object.fromEntries(agents.map((a) => [a.id, a.hostname]));
  const router = devices.find((d) => d.isRouter);

  return (
    <div className="space-y-12">
      <PageHeader
        title="Devices"
        description={
          <>
            Everything connected to your router, consoles and phones included,
            found by asking every address who's there (<Term id="arp">ARP</Term>
            ). Rename devices so they're easy to recognize.
          </>
        }
        actions={
          <p className="text-sm text-zinc-500">
            <span className="font-semibold text-zinc-900 tabular-nums dark:text-zinc-100">
              {online}
            </span>{" "}
            online · {devices.length} seen
          </p>
        }
      />

      <section>
        {devices.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-zinc-300 p-6 dark:border-zinc-700">
            <p className="text-sm font-medium">No devices yet</p>
            <p className="mt-1 text-sm text-zinc-500">
              The first sweep runs when the hub starts and takes a few seconds.
            </p>
          </div>
        ) : (
          <div className="panel overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-zinc-950/[0.06] text-xs text-zinc-500 dark:border-white/[0.06]">
                <tr>
                  <th className="px-5 py-3 font-medium">Device</th>
                  <th className="px-5 py-3 font-medium">
                    <span className="inline-flex items-center gap-1">
                      IP <InfoTip id="ip" />
                    </span>
                  </th>
                  <th className="px-5 py-3 font-medium">
                    <span className="inline-flex items-center gap-1">
                      LAN ping, last 10 min <InfoTip id="lanPing" />
                    </span>
                  </th>
                  <th className="px-5 py-3 font-medium">Last seen</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-950/[0.05] dark:divide-white/[0.05]">
                {devices.map((d) => (
                  <DeviceRow
                    key={d.mac}
                    device={d}
                    router={router}
                    agentName={agentNames[d.agentId]}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <AgentList agents={agents} />
    </div>
  );
}

const QUALITY: Record<Quality, { color: string; text: string }> = {
  good: { color: "var(--status-good)", text: "good" },
  fair: { color: "var(--status-warning)", text: "fair" },
  slow: { color: "var(--status-critical)", text: "slow" },
  none: { color: "transparent", text: "" },
};

function DeviceRow({
  device: d,
  router,
  agentName,
}: {
  device: LanDevice;
  router?: LanDevice;
  agentName?: string;
}) {
  const [editing, setEditing] = useState(false);
  const [chart, setChart] = useState(false);
  const canChart = d.answersPing && !d.isHub;
  const ping = pingSummary(d);
  const quality = QUALITY[ping.quality];
  const sub = details(d);

  return (
    <>
      <tr className={d.online ? "" : "text-zinc-400"}>
        <td className="px-5 py-3.5">
          <div className="flex items-start gap-2">
            <span
              role="img"
              aria-label={d.online ? "online" : "offline"}
              className={`mt-1.5 size-2 shrink-0 rounded-full ${
                d.online ? "bg-green-500" : "bg-zinc-400"
              }`}
            />
            <div className="min-w-0">
              {editing ? (
                <RenameForm device={d} onDone={() => setEditing(false)} />
              ) : (
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span className="font-medium">{displayName(d)}</span>
                  {d.isHub && <Badge>this hub</Badge>}
                  {agentName && <Badge>agent</Badge>}
                  <button
                    type="button"
                    onClick={() => setEditing(true)}
                    className="text-xs text-[var(--accent)] hover:underline"
                  >
                    Rename
                  </button>
                </div>
              )}
              {sub && (
                <p className="flex items-center gap-1 text-xs text-zinc-500">
                  {sub}
                  {d.privateMac && <InfoTip id="privateMac" />}
                </p>
              )}
              <p className="font-mono text-xs text-zinc-400">{d.mac}</p>
            </div>
          </div>
        </td>
        <td className="px-5 py-3.5 font-mono text-xs">{d.ip}</td>
        <td className="px-5 py-3.5">
          <div className="flex items-center gap-2">
            {ping.quality !== "none" && (
              <span
                aria-hidden
                className="size-2 shrink-0 rounded-full"
                style={{ background: quality.color }}
              />
            )}
            <span className="font-medium tabular-nums">{ping.label}</span>
            {quality.text && (
              <span className="text-xs text-zinc-500">{quality.text}</span>
            )}
          </div>
          <p className="text-xs text-zinc-500">{ping.detail}</p>
          {canChart && (
            <button
              type="button"
              aria-expanded={chart}
              onClick={() => setChart((c) => !c)}
              className="mt-1 text-xs font-medium text-[var(--accent)] hover:underline"
            >
              {chart ? "Hide ping chart" : "Ping chart"}
            </button>
          )}
        </td>
        <td className="px-5 py-3.5 tabular-nums">
          {d.lastSeen ? timeAgo(timestampDate(d.lastSeen)) : "—"}
        </td>
      </tr>
      {chart && (
        <tr>
          <td
            colSpan={4}
            className="bg-zinc-50/60 px-5 py-4 dark:bg-zinc-950/40"
          >
            <DevicePingChart device={d} router={router} />
          </td>
        </tr>
      )}
    </>
  );
}

function RenameForm({
  device: d,
  onDone,
}: {
  device: LanDevice;
  onDone: () => void;
}) {
  const fetcher = useFetcher();
  const input = useRef<HTMLInputElement>(null);
  const submitted = useRef(false);

  useEffect(() => input.current?.focus(), []);
  useEffect(() => {
    if (submitted.current && fetcher.state === "idle") onDone();
  }, [fetcher.state, onDone]);

  return (
    <fetcher.Form
      method="post"
      className="flex gap-1"
      onSubmit={() => {
        submitted.current = true;
      }}
    >
      <input type="hidden" name="mac" value={d.mac} />
      <input
        ref={input}
        name="nickname"
        aria-label={`Name for ${displayName(d)}`}
        defaultValue={d.nickname}
        placeholder={d.hostname || "e.g. Xbox"}
        maxLength={64}
        onKeyDown={(e) => e.key === "Escape" && onDone()}
        className="w-44 rounded-md border border-zinc-300 px-2 py-0.5 text-sm dark:border-zinc-700 dark:bg-zinc-950"
      />
      <button
        type="submit"
        disabled={fetcher.state !== "idle"}
        className="rounded bg-zinc-900 px-2 py-0.5 text-xs text-white disabled:opacity-50 dark:bg-zinc-100 dark:text-zinc-900"
      >
        Save
      </button>
      <button
        type="button"
        onClick={onDone}
        className="px-1 text-xs text-zinc-500 hover:underline"
      >
        Cancel
      </button>
    </fetcher.Form>
  );
}

function Badge({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded bg-zinc-100 px-1.5 py-0.5 text-xs text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">
      {children}
    </span>
  );
}

function AgentList({
  agents,
}: {
  agents: Route.ComponentProps["loaderData"]["agents"];
}) {
  return (
    <section>
      <SectionHeader
        title={<Term id="agent">Computers running the agent</Term>}
        description="These report their own download and upload, so NetWatch can tell who was busy when lag hit."
      />
      {agents.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-zinc-300 p-6 text-sm text-zinc-500 dark:border-zinc-700">
          No agents have checked in yet.
        </div>
      ) : (
        <div className="panel overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-zinc-950/[0.06] text-xs text-zinc-500 dark:border-white/[0.06]">
              <tr>
                <th className="px-5 py-3 font-medium">Computer</th>
                <th className="px-5 py-3 font-medium">System</th>
                <th className="px-5 py-3 font-medium">IP</th>
                <th className="px-5 py-3 font-medium">
                  <span className="inline-flex items-center gap-1">
                    Wi-Fi signal <InfoTip id="wifiSignal" />
                  </span>
                </th>
                <th className="px-5 py-3 font-medium">Last report</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-950/[0.05] dark:divide-white/[0.05]">
              {agents.map((agent) => (
                <tr key={agent.id}>
                  <td className="px-5 py-3.5">
                    <div className="flex items-center gap-2 font-medium">
                      <span
                        role="img"
                        aria-label={agent.online ? "online" : "offline"}
                        className={`size-2 rounded-full ${
                          agent.online ? "bg-green-500" : "bg-zinc-400"
                        }`}
                      />
                      {agent.hostname}
                    </div>
                  </td>
                  <td className="px-5 py-3.5 text-zinc-600 dark:text-zinc-400">
                    {agent.os}/{agent.arch}
                    <span className="ml-2 text-xs text-zinc-400">
                      {agent.agentVersion}
                    </span>
                  </td>
                  <td className="px-5 py-3.5 font-mono text-xs text-zinc-600 dark:text-zinc-400">
                    {agent.ips.filter((ip) => !ip.includes(":")).join(", ")}
                  </td>
                  <td className="px-5 py-3.5">
                    <WifiCell wifi={agent.wifi} />
                  </td>
                  <td className="px-5 py-3.5 text-zinc-600 tabular-nums dark:text-zinc-400">
                    {agent.lastSeen
                      ? timeAgo(timestampDate(agent.lastSeen))
                      : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function WifiCell({ wifi }: { wifi?: WifiStatus }) {
  if (!wifi) {
    return <span className="text-xs text-zinc-400">Wired, or no reading</span>;
  }
  const quality = wifiQuality(wifi.signalDbm);
  return (
    <div>
      <div className="flex items-center gap-2">
        <span
          aria-hidden
          className="size-2 shrink-0 rounded-full"
          style={{ background: WIFI_QUALITY_COLOR[quality] }}
        />
        <span className="font-medium">{quality}</span>
      </div>
      <p className="text-xs text-zinc-500">{describeWifi(wifi)}</p>
    </div>
  );
}
