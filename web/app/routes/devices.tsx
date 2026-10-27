import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useEffect, useRef, useState } from "react";
import { useFetcher } from "react-router";
import {
  details,
  displayName,
  pingSummary,
  type Quality,
} from "~/features/devices/describe";
import type { LanDevice } from "~/gen/netwatch/v1/dashboard_pb";
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

  return (
    <main className="space-y-10">
      <section className="space-y-3">
        <div>
          <h2 className="text-lg font-semibold">
            Every device on your network
            <span className="ml-2 text-sm font-normal text-gray-500">
              {online} online, {devices.length} seen
            </span>
          </h2>
          <p className="text-sm text-gray-500">
            Found by asking every address on the network who's there (ARP), so
            even devices that ignore pings show up. LAN ping is the round trip
            from the hub to the device: fast and steady means good Wi-Fi.
          </p>
        </div>

        {devices.length === 0 ? (
          <p className="rounded-xl border border-dashed border-gray-300 p-5 text-sm text-gray-500 dark:border-gray-700">
            No devices yet. The first sweep runs when the hub starts and takes a
            few seconds.
          </p>
        ) : (
          <div className="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-gray-200 text-gray-500 dark:border-gray-800">
                <tr>
                  <th className="px-4 py-2 font-medium">Device</th>
                  <th className="px-4 py-2 font-medium">IP</th>
                  <th className="px-4 py-2 font-medium">LAN ping, 10 min</th>
                  <th className="px-4 py-2 font-medium">Last seen</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
                {devices.map((d) => (
                  <DeviceRow
                    key={d.mac}
                    device={d}
                    agentName={agentNames[d.agentId]}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <AgentList agents={agents} />
    </main>
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
  agentName,
}: {
  device: LanDevice;
  agentName?: string;
}) {
  const [editing, setEditing] = useState(false);
  const ping = pingSummary(d);
  const quality = QUALITY[ping.quality];
  const sub = details(d);

  return (
    <tr className={d.online ? "" : "text-gray-400"}>
      <td className="px-4 py-3">
        <div className="flex items-start gap-2">
          <span
            role="img"
            aria-label={d.online ? "online" : "offline"}
            className={`mt-1.5 size-2 shrink-0 rounded-full ${
              d.online ? "bg-green-500" : "bg-gray-400"
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
                  className="text-xs text-blue-600 hover:underline dark:text-blue-400"
                >
                  Rename
                </button>
              </div>
            )}
            {sub && <p className="text-xs text-gray-500">{sub}</p>}
            <p className="font-mono text-xs text-gray-400">{d.mac}</p>
          </div>
        </div>
      </td>
      <td className="px-4 py-3 font-mono text-xs">{d.ip}</td>
      <td className="px-4 py-3">
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
            <span className="text-xs text-gray-500">{quality.text}</span>
          )}
        </div>
        <p className="text-xs text-gray-500">{ping.detail}</p>
      </td>
      <td className="px-4 py-3 tabular-nums">
        {d.lastSeen ? timeAgo(timestampDate(d.lastSeen)) : "—"}
      </td>
    </tr>
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
        className="w-40 rounded border border-gray-300 px-2 py-0.5 text-sm dark:border-gray-700 dark:bg-gray-950"
      />
      <button
        type="submit"
        disabled={fetcher.state !== "idle"}
        className="rounded bg-gray-900 px-2 py-0.5 text-xs text-white disabled:opacity-50 dark:bg-gray-100 dark:text-gray-900"
      >
        Save
      </button>
      <button
        type="button"
        onClick={onDone}
        className="px-1 text-xs text-gray-500 hover:underline"
      >
        Cancel
      </button>
    </fetcher.Form>
  );
}

function Badge({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-600 dark:bg-gray-800 dark:text-gray-300">
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
    <section className="space-y-3">
      <h2 className="text-lg font-semibold">Computers running the agent</h2>
      {agents.length === 0 ? (
        <p className="text-sm text-gray-500">No agents have checked in yet.</p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-gray-200 text-gray-500 dark:border-gray-800">
              <tr>
                <th className="px-4 py-2 font-medium">Computer</th>
                <th className="px-4 py-2 font-medium">System</th>
                <th className="px-4 py-2 font-medium">IP</th>
                <th className="px-4 py-2 font-medium">Last report</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
              {agents.map((agent) => (
                <tr key={agent.id}>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2 font-medium">
                      <span
                        role="img"
                        aria-label={agent.online ? "online" : "offline"}
                        className={`size-2 rounded-full ${
                          agent.online ? "bg-green-500" : "bg-gray-400"
                        }`}
                      />
                      {agent.hostname}
                    </div>
                  </td>
                  <td className="px-4 py-3 text-gray-600 dark:text-gray-400">
                    {agent.os}/{agent.arch}
                    <span className="ml-2 text-xs text-gray-400">
                      {agent.agentVersion}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-600 dark:text-gray-400">
                    {agent.ips.filter((ip) => !ip.includes(":")).join(", ")}
                  </td>
                  <td className="px-4 py-3 text-gray-600 tabular-nums dark:text-gray-400">
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
