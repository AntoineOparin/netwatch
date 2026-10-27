import { Link } from "react-router";
import { dashboard, usePolling } from "~/lib/api";
import type { Route } from "./+types/overview";

export async function clientLoader() {
  const { agents } = await dashboard().listAgents({});
  return {
    total: agents.length,
    online: agents.filter((a) => a.online).length,
  };
}

export default function Overview({ loaderData }: Route.ComponentProps) {
  usePolling(5000);
  const { total, online } = loaderData;

  return (
    <main className="grid gap-4 sm:grid-cols-2">
      <section className="rounded-xl border border-gray-200 bg-white p-5 dark:border-gray-800 dark:bg-gray-900">
        <h2 className="text-sm font-medium text-gray-500">Agents online</h2>
        <p className="mt-2 text-4xl font-semibold tabular-nums">
          {online}
          <span className="text-xl text-gray-400"> / {total}</span>
        </p>
        <Link
          to="/devices"
          className="mt-3 inline-block text-sm text-blue-600 hover:underline dark:text-blue-400"
        >
          See devices →
        </Link>
      </section>

      {total === 0 && (
        <section className="rounded-xl border border-dashed border-gray-300 p-5 text-sm dark:border-gray-700">
          <h2 className="font-medium">No agents yet</h2>
          <p className="mt-2 text-gray-500">
            On each computer, run the agent pointed at this hub:
          </p>
          <pre className="mt-2 overflow-x-auto rounded bg-gray-100 p-2 dark:bg-gray-800">
            <code>agent -hub {window.location.origin}</code>
          </pre>
        </section>
      )}
    </main>
  );
}
