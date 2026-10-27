import { Link } from "react-router";
import { PageHeader, SectionHeader } from "~/components/page-header";
import { GLOSSARY, type Term, type TermId } from "~/features/learn/glossary";

export function meta() {
  return [{ title: "Learn · NetWatch" }];
}

export default function Learn() {
  return (
    <>
      <PageHeader
        title="Learn"
        description="New to networking? This page explains what NetWatch measures and how to read it, no background needed."
      />
      <div className="space-y-14">
        <Path />
        <Tour />
        <HubAndAgents />
        <Visibility />
        <Glossary />
      </div>
    </>
  );
}

const STOPS = [
  {
    name: "Your devices",
    detail: "Xbox, phones, laptops, TV",
    color: null,
  },
  {
    name: "Your router",
    detail: "The box every device connects to. Slow here? It's your Wi-Fi.",
    color: "var(--series-1)",
  },
  {
    name: "Your ISP",
    detail:
      "Your provider's first router. Slow only from here? The connection is congested.",
    color: "var(--series-2)",
  },
  {
    name: "The internet",
    detail: "Game servers, Netflix, 1.1.1.1. Slow only here? It's further out.",
    color: "var(--series-3)",
  },
];

// The one idea everything else builds on: data passes several stops, and
// the first slow one is where the trouble is.
function Path() {
  return (
    <section>
      <SectionHeader
        title="How your data travels"
        description="Everything you do online goes through the same stops. NetWatch pings each colored stop once a second. The first one that's slow is where the problem starts, because every stop after it inherits the delay."
      />
      <ol className="grid gap-3 md:grid-cols-4">
        {STOPS.map((s, i) => (
          <li key={s.name} className="relative">
            <div className="panel h-full p-4">
              <div className="flex items-center gap-2">
                <span
                  aria-hidden
                  className="flex size-6 items-center justify-center rounded-md text-xs font-semibold text-white"
                  style={{ background: s.color ?? "var(--chart-axis)" }}
                >
                  {i + 1}
                </span>
                <span className="font-semibold">{s.name}</span>
              </div>
              <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-400">
                {s.detail}
              </p>
            </div>
            {i < STOPS.length - 1 && (
              <span
                aria-hidden
                className="absolute top-1/2 -right-3 z-10 hidden -translate-y-1/2 text-zinc-300 md:block dark:text-zinc-600"
              >
                →
              </span>
            )}
          </li>
        ))}
      </ol>
    </section>
  );
}

const TOUR = [
  {
    title: "Right now",
    body: "The one-sentence answer: is the connection healthy, and if not, which stop is struggling.",
  },
  {
    title: "Ping cards",
    body: "How long a message takes to reach each stop and come back, plus how jumpy it is (jitter) and how many go missing (lost). Lower is better.",
  },
  {
    title: "Ping over time",
    body: "The same three stops as lines. When the orange and green lines rise but the blue one stays low, something is crowding the connection. Shaded stretches are lag incidents.",
  },
  {
    title: "Recent lag",
    body: "Each time ping jumped well above normal, in plain words, with the computers (and apps) that were busy at that moment.",
  },
  {
    title: "Who's using the internet",
    body: "Download and upload for every computer running the agent. A big upload is the usual reason everyone's ping rises.",
  },
];

function Tour() {
  return (
    <section>
      <SectionHeader
        title="Reading the overview"
        description="From top to bottom:"
      />
      <ol className="panel divide-y divide-zinc-950/[0.06] dark:divide-white/[0.06]">
        {TOUR.map((t, i) => (
          <li key={t.title} className="flex gap-4 p-5">
            <span className="w-5 shrink-0 font-mono text-sm text-zinc-400 tabular-nums">
              {i + 1}
            </span>
            <div>
              <p className="font-medium">{t.title}</p>
              <p className="mt-0.5 max-w-2xl text-sm text-zinc-600 dark:text-zinc-400">
                {t.body}
              </p>
            </div>
          </li>
        ))}
      </ol>
      <p className="mt-3 text-sm text-zinc-500">
        Ready to look?{" "}
        <Link
          to="/"
          className="font-medium text-[var(--accent)] hover:underline"
        >
          Open the overview
        </Link>
        .
      </p>
    </section>
  );
}

function HubAndAgents() {
  return (
    <section>
      <SectionHeader title="Hub and agents" />
      <div className="grid gap-4 md:grid-cols-2">
        {(["hub", "agent"] as const).map((id) => {
          const t: Term = GLOSSARY[id];
          return (
            <div key={id} className="panel p-5">
              <p className="font-semibold">{t.term}</p>
              <p className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">
                {t.short}
              </p>
              {t.more && (
                <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-400">
                  {t.more}
                </p>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function Visibility() {
  const rows = [
    ["Lag on your connection", "Yes", "Yes"],
    ["Shows up in the device list", "Yes", "Yes"],
    ["Wi-Fi quality (LAN ping)", "Yes", "If it answers pings"],
    ["How much it downloads and uploads", "Yes", "No"],
    ["Which apps (Macs only)", "Yes", "No"],
  ];
  return (
    <section>
      <SectionHeader
        title="What NetWatch can and can't see"
        description="A computer on a home network only sees its own traffic, so devices that can't run the agent are visible but not measured."
      />
      <div className="panel overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-zinc-950/[0.06] text-xs text-zinc-500 dark:border-white/[0.06]">
            <tr>
              <th className="px-5 py-3 font-medium" />
              <th className="px-5 py-3 font-medium">
                Computers with the agent
              </th>
              <th className="px-5 py-3 font-medium">Xbox, phones, TV</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-950/[0.05] dark:divide-white/[0.05]">
            {rows.map(([what, agent, other]) => (
              <tr key={what}>
                <th
                  scope="row"
                  className="px-5 py-3 font-normal text-zinc-600 dark:text-zinc-400"
                >
                  {what}
                </th>
                <td className="px-5 py-3">{agent}</td>
                <td
                  className={`px-5 py-3 ${other === "No" ? "text-zinc-400" : ""}`}
                >
                  {other}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-3 max-w-2xl text-sm text-zinc-500">
        When lag hits and no computer with an agent was busy, NetWatch says so,
        which usually points at one of the devices it can't measure.
      </p>
    </section>
  );
}

function Glossary() {
  const ids = Object.keys(GLOSSARY) as TermId[];
  return (
    <section>
      <SectionHeader
        title="Glossary"
        description="Every term on the dashboard. The ⓘ buttons next to terms link here."
      />
      <dl className="grid gap-3 lg:grid-cols-2">
        {ids.map((id) => {
          const t: Term = GLOSSARY[id];
          return (
            <div
              key={id}
              id={id}
              className="panel scroll-mt-24 p-5 target:ring-2 target:ring-[var(--accent)]"
            >
              <dt className="font-semibold">{t.term}</dt>
              <dd className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">
                {t.short}
                {t.more && <span className="mt-2 block">{t.more}</span>}
              </dd>
            </div>
          );
        })}
      </dl>
    </section>
  );
}
