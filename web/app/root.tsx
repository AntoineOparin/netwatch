import {
  isRouteErrorResponse,
  Links,
  Meta,
  NavLink,
  Outlet,
  Scripts,
  ScrollRestoration,
} from "react-router";

import type { Route } from "./+types/root";
import "./app.css";

export const links: Route.LinksFunction = () => [
  { rel: "preconnect", href: "https://fonts.googleapis.com" },
  {
    rel: "preconnect",
    href: "https://fonts.gstatic.com",
    crossOrigin: "anonymous",
  },
  {
    rel: "stylesheet",
    href: "https://fonts.googleapis.com/css2?family=Inter:ital,opsz,wght@0,14..32,100..900;1,14..32,100..900&display=swap",
  },
];

export function meta() {
  return [{ title: "NetWatch" }];
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

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  `rounded-md px-3 py-1.5 text-sm font-medium ${
    isActive
      ? "bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900"
      : "text-gray-600 hover:bg-gray-200 dark:text-gray-400 dark:hover:bg-gray-800"
  }`;

export default function App() {
  return (
    <div className="mx-auto max-w-5xl px-4 py-6 sm:px-8">
      <header className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">NetWatch</h1>
          <p className="text-sm text-gray-500">
            Is the internet dropping, and who's using it?
          </p>
        </div>
        <nav className="flex gap-1">
          <NavLink to="/" end className={navLinkClass}>
            Overview
          </NavLink>
          <NavLink to="/devices" className={navLinkClass}>
            Devices
          </NavLink>
        </nav>
      </header>
      <Outlet />
    </div>
  );
}

// Shown while the first clientLoader runs.
export function HydrateFallback() {
  return <p className="p-8 text-sm text-gray-500">Loading…</p>;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let message = "Oops!";
  let details = "An unexpected error occurred.";
  let stack: string | undefined;

  if (isRouteErrorResponse(error)) {
    message = error.status === 404 ? "404" : "Error";
    details =
      error.status === 404
        ? "The requested page could not be found."
        : error.statusText || details;
  } else if (error instanceof Error) {
    // Usually "hub unreachable": worth showing in prod too.
    details = error.message;
    if (import.meta.env.DEV) stack = error.stack;
  }

  return (
    <main className="container mx-auto p-4 pt-16">
      <h1>{message}</h1>
      <p>{details}</p>
      {stack && (
        <pre className="w-full overflow-x-auto p-4">
          <code>{stack}</code>
        </pre>
      )}
    </main>
  );
}
