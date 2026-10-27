import { type Client, createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { useEffect } from "react";
import { useRevalidator } from "react-router";
import { DashboardService } from "~/gen/netwatch/v1/dashboard_pb";

let dashboardClient: Client<typeof DashboardService> | undefined;

// Created lazily: the SPA build pre-renders index.html in Node, where there is
// no window. Same origin works for both the hub and the Vite dev proxy.
export function dashboard() {
  dashboardClient ??= createClient(
    DashboardService,
    createConnectTransport({ baseUrl: window.location.origin }),
  );
  return dashboardClient;
}

// Re-runs the current route's clientLoader every `ms` milliseconds. Live
// streaming (SSE) replaces this once there is time-series data to push.
export function usePolling(ms: number) {
  const { revalidate } = useRevalidator();
  useEffect(() => {
    const id = setInterval(revalidate, ms);
    return () => clearInterval(id);
  }, [revalidate, ms]);
}

export function timeAgo(date: Date, now = new Date()) {
  const seconds = Math.max(
    0,
    Math.round((now.getTime() - date.getTime()) / 1000),
  );
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.round(hours / 24)}d ago`;
}
