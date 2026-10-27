import { timestampDate } from "@bufbuild/protobuf/wkt";
import { dashboard, timeAgo, usePolling } from "~/lib/api";
import type { Route } from "./+types/devices";

export function meta() {
  return [{ title: "Devices · NetWatch" }];
}

export async function clientLoader() {
  const { agents } = await dashboard().listAgents({});
  return { agents };
}

export default function Devices({ loaderData }: Route.ComponentProps) {
  usePolling(5000);
  const { agents } = loaderData;

  return (
    <main className="space-y-3">
      <h2 className="text-lg font-semibold">Computers running an agent</h2>
      {agents.length === 0 ? (
        <p className="text-sm text-gray-500">No agents have checked in yet.</p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-gray-200 text-gray-500 dark:border-gray-800">
              <tr>
                <th className="px-4 py-2 font-medium">Device</th>
                <th className="px-4 py-2 font-medium">System</th>
                <th className="px-4 py-2 font-medium">IP</th>
                <th className="px-4 py-2 font-medium">Last seen</th>
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
                    {agent.ips.join(", ")}
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
    </main>
  );
}
