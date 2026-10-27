import { index, type RouteConfig, route } from "@react-router/dev/routes";

// Every URL in the app is declared here. Route modules live in app/routes/.
export default [
  index("routes/overview.tsx"),
  route("incidents", "routes/incidents.tsx"),
  route("devices", "routes/devices.tsx"),
  route("learn", "routes/learn.tsx"),
] satisfies RouteConfig;
