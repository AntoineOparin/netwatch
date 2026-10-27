import { useEffect, useState } from "react";
import {
  isRouteErrorResponse,
  Link,
  Links,
  Meta,
  NavLink,
  Outlet,
  Scripts,
  ScrollRestoration,
} from "react-router";
import {
  BookIcon,
  DevicesIcon,
  Logo,
  PulseIcon,
  SidebarIcon,
  SpikeIcon,
} from "~/components/icons";
import type { Route } from "./+types/root";
import "./app.css";

const FAVICON = `data:image/svg+xml,${encodeURIComponent(
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="9" fill="#18181b"/><path d="M6 18h5l3-7 4 10 3-6h5" fill="none" stroke="#fff" stroke-width="2.25" stroke-linecap="round" stroke-linejoin="round"/></svg>',
)}`;

export const links: Route.LinksFunction = () => [
  { rel: "icon", href: FAVICON, type: "image/svg+xml" },
  { rel: "preconnect", href: "https://fonts.googleapis.com" },
  {
    rel: "preconnect",
    href: "https://fonts.gstatic.com",
    crossOrigin: "anonymous",
  },
  {
    rel: "stylesheet",
    href: "https://fonts.googleapis.com/css2?family=Geist:wght@400..700&family=Geist+Mono:wght@400..600&display=swap",
  },
];

export function meta() {
  return [
    { title: "NetWatch" },
    {
      name: "description",
      content:
        "Is the home internet dropping, and who's using it? A live view of your connection, every device, and what was busy when lag hit.",
    },
  ];
}

export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body>
        {children}
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

const NAV = [
  { to: "/", label: "Overview", icon: PulseIcon, end: true },
  { to: "/incidents", label: "Lag incidents", icon: SpikeIcon },
  { to: "/devices", label: "Devices", icon: DevicesIcon },
  { to: "/learn", label: "Learn", icon: BookIcon },
];

function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <Link
      to="/"
      aria-label="NetWatch home"
      className="flex items-center gap-2.5 rounded-lg"
    >
      <Logo />
      {!compact && (
        <span className="text-[15px] font-semibold tracking-tight">
          NetWatch
        </span>
      )}
    </Link>
  );
}

const COLLAPSED_KEY = "netwatch.sidebar.collapsed";

// Whether the desktop sidebar is folded into an icon rail. Remembered per
// browser; storage can be unavailable (private windows), so it's optional.
function useCollapsed() {
  const [collapsed, setCollapsed] = useState(false);
  useEffect(() => {
    try {
      setCollapsed(localStorage.getItem(COLLAPSED_KEY) === "1");
    } catch {}
  }, []);
  const toggle = () =>
    setCollapsed((c) => {
      try {
        localStorage.setItem(COLLAPSED_KEY, c ? "0" : "1");
      } catch {}
      return !c;
    });
  return [collapsed, toggle] as const;
}

function Shell({ children }: { children: React.ReactNode }) {
  const [collapsed, toggle] = useCollapsed();
  return (
    <div
      className={`min-h-dvh transition-[grid-template-columns] duration-200 lg:grid ${
        collapsed
          ? "lg:grid-cols-[4.5rem_minmax(0,1fr)]"
          : "lg:grid-cols-[15rem_minmax(0,1fr)]"
      }`}
    >
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-lg focus:bg-white focus:px-3 focus:py-2 focus:text-sm focus:shadow dark:focus:bg-zinc-900"
      >
        Skip to content
      </a>

      {/* Desktop: a sidebar that stays put while the page scrolls, and folds
          into an icon rail. */}
      <aside
        className={`sticky top-0 hidden h-dvh flex-col overflow-hidden border-r border-zinc-950/[0.06] py-6 lg:flex dark:border-white/[0.06] ${
          collapsed ? "items-center px-3" : "px-4"
        }`}
      >
        <div className={collapsed ? "" : "px-2"}>
          <Brand compact={collapsed} />
          {!collapsed && (
            <p className="mt-2 text-xs leading-relaxed text-zinc-500 dark:text-zinc-400">
              Is the internet dropping, and who's using it?
            </p>
          )}
        </div>
        <nav
          aria-label="Main"
          className={`mt-8 flex flex-col gap-0.5 ${collapsed ? "" : "self-stretch"}`}
        >
          {NAV.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              title={collapsed ? label : undefined}
              className={({ isActive }) =>
                `group flex items-center gap-3 rounded-lg py-2 text-sm font-medium transition-colors ${
                  collapsed ? "justify-center px-2.5" : "px-2.5"
                } ${
                  isActive
                    ? "bg-white text-zinc-900 shadow-sm ring-1 shadow-zinc-950/[0.04] ring-zinc-950/[0.06] dark:bg-zinc-900 dark:text-zinc-50 dark:ring-white/[0.08]"
                    : "text-zinc-600 hover:bg-zinc-950/[0.04] hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-white/[0.04] dark:hover:text-zinc-100"
                }`
              }
            >
              {({ isActive }) => (
                <>
                  <Icon
                    className={`size-[18px] shrink-0 ${isActive ? "text-[var(--accent)]" : "text-zinc-400 group-hover:text-zinc-600 dark:text-zinc-500 dark:group-hover:text-zinc-300"}`}
                  />
                  <span className={collapsed ? "sr-only" : ""}>{label}</span>
                </>
              )}
            </NavLink>
          ))}
        </nav>
        <div
          className={`mt-auto flex flex-col gap-4 ${collapsed ? "items-center" : "self-stretch"}`}
        >
          {!collapsed && (
            <p className="px-2 text-xs leading-relaxed text-zinc-500 dark:text-zinc-400">
              Runs entirely on your home network. Nothing is sent anywhere else.
            </p>
          )}
          <button
            type="button"
            onClick={toggle}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            className="flex items-center gap-3 rounded-lg px-2.5 py-2 text-sm font-medium text-zinc-500 transition-colors hover:bg-zinc-950/[0.04] hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-white/[0.04] dark:hover:text-zinc-100"
          >
            <SidebarIcon className="size-[18px] shrink-0" />
            {!collapsed && "Collapse"}
          </button>
        </div>
      </aside>

      {/* Mobile: brand plus a row of tabs. */}
      <header className="sticky top-0 z-40 border-b border-zinc-950/[0.06] bg-zinc-50/90 backdrop-blur lg:hidden dark:border-white/[0.06] dark:bg-zinc-950/90">
        <div className="flex items-center px-4 pt-3">
          <Brand />
        </div>
        <nav aria-label="Main" className="flex gap-1 overflow-x-auto px-3 py-2">
          {NAV.map(({ to, label, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                `shrink-0 rounded-lg px-3 py-1.5 text-sm font-medium ${
                  isActive
                    ? "bg-zinc-900 text-white dark:bg-zinc-100 dark:text-zinc-900"
                    : "text-zinc-600 dark:text-zinc-400"
                }`
              }
            >
              {label}
            </NavLink>
          ))}
        </nav>
      </header>

      <main
        id="main"
        className="mx-auto w-full max-w-[1280px] px-4 py-8 sm:px-8 lg:py-10"
      >
        {children}
      </main>
    </div>
  );
}

export default function App() {
  return (
    <Shell>
      <Outlet />
    </Shell>
  );
}

// Shown while a page's first data loads: the rough shape of a page, so
// nothing jumps when it arrives.
export function HydrateFallback() {
  return (
    <Shell>
      <div
        role="status"
        aria-busy="true"
        aria-label="Loading"
        className="animate-pulse motion-reduce:animate-none"
      >
        <div className="h-7 w-48 rounded-lg bg-zinc-200 dark:bg-zinc-800" />
        <div className="mt-3 h-4 w-80 max-w-full rounded bg-zinc-200/70 dark:bg-zinc-800/70" />
        <div className="mt-10 grid gap-4 xl:grid-cols-3">
          <div className="h-28 rounded-2xl bg-zinc-200/70 xl:col-span-2 dark:bg-zinc-800/70" />
          <div className="h-28 rounded-2xl bg-zinc-200/70 dark:bg-zinc-800/70" />
        </div>
        <div className="mt-4 h-72 rounded-2xl bg-zinc-200/70 dark:bg-zinc-800/70" />
      </div>
    </Shell>
  );
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let title = "Can't reach the hub";
  let details =
    "The dashboard couldn't get data from the NetWatch hub. Check that the hub is still running, then try again.";
  let stack: string | undefined;

  if (isRouteErrorResponse(error)) {
    title = error.status === 404 ? "Page not found" : `Error ${error.status}`;
    details =
      error.status === 404
        ? "There's nothing at this address."
        : error.statusText || details;
  } else if (import.meta.env.DEV && error instanceof Error) {
    stack = error.stack;
  }

  return (
    <Shell>
      <div className="max-w-lg py-16">
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        <p className="mt-2 text-zinc-600 dark:text-zinc-400">{details}</p>
        <div className="mt-6 flex gap-3">
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-zinc-700 active:scale-[0.98] dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-white"
          >
            Try again
          </button>
          <Link
            to="/"
            className="rounded-lg px-4 py-2 text-sm font-medium text-zinc-600 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
          >
            Go to overview
          </Link>
        </div>
        {stack && (
          <pre className="mt-8 overflow-x-auto rounded-xl bg-zinc-100 p-4 text-xs dark:bg-zinc-900">
            <code>{stack}</code>
          </pre>
        )}
      </div>
    </Shell>
  );
}
