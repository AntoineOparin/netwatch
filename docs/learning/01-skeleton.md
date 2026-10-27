# M1: Skeleton

## What got built

- The agent registers with the hub, then heartbeats on an interval the hub picks.
- The hub keeps an in-memory registry and marks an agent offline after 3 missed heartbeats.
- The dashboard lists the agents. It's built as a static SPA and compiled into the hub binary.

## Concepts

### Schema-first APIs (protobuf + Connect)

The contract lives in `proto/netwatch/v1/*.proto`. `buf generate` turns it into:

- Go interfaces the hub implements (`gen/.../netwatchv1connect`)
- a typed Go client the agent uses
- a typed TypeScript client the dashboard uses (`web/app/gen`)

Rename a field and both sides fail to compile until they're fixed. That safety is what the schema buys you. Connect speaks plain HTTP: try `curl -X POST -H 'Content-Type: application/json' -d '{}' localhost:8080/netwatch.v1.DashboardService/ListAgents`.

### Liveness: heartbeats and "who owns the state"

The hub's registry is in memory only. When the hub restarts, it forgets everyone. Instead of persisting the registry, the system recovers itself:

1. The agent sends a heartbeat.
2. The hub answers `NOT_FOUND`.
3. The agent registers again.

The agent owns its identity (a random ID saved to disk on first run), so a re-register updates the same entry instead of creating a duplicate. This pattern is called *soft state*: state that gets rebuilt from periodic refreshes. It's the same idea behind DHCP leases and the ARP cache (M5).

"Online" means seen within 3× the heartbeat interval. One dropped packet shouldn't flip a device offline.

### Agents push, the hub doesn't pull

Laptops sleep, change IPs, and sit behind firewalls. The hub can't reliably connect *to* them, but they can always connect *out* to the hub. So agents push.

### Shipping a web UI inside a Go binary

React Router in SPA mode builds plain static files. `go:embed` bakes them into the hub, so deploying means copying one file. Client-side routes like `/devices` don't exist as files on disk, so the hub falls back to `index.html` for paths without an extension and lets the router take over in the browser.

## Try it

- Kill the hub while an agent runs, then start it again. The agent log shows `registering again`.
- Kill the agent. After about 15 s the dashboard shows it offline.
