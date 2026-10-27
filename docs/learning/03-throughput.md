# M3: Who's using bandwidth

## What got built

- Each agent measures its computer's download and upload speed once a second and reports it to the hub every 2 seconds.
- The hub keeps everything in SQLite.
- The dashboard shows a "who's using it now" table, plus download and upload charts with one line per computer.

## Concepts

### Bytes per second from interface counters

The OS keeps running totals of bytes in and out of every network interface (`netstat -ib` shows them on macOS). A rate is just the difference between two readings divided by the time between them. Two details:

- **Pick the right interface.** Only the *uplink*, the interface the default route leaves through (`en0` here), counts. VPN tunnels (`utun*`) carry the same traffic again in encrypted form, so summing every interface would double count.
- **Counters can reset** when the interface restarts or the machine sleeps. A reading lower than the previous one means "skip this interval", not "negative traffic".

Testing lesson: my first "download test" showed nothing, and I assumed the collector was broken. It wasn't. Cloudflare had answered HTTP 403 to a 200 MB request, so nothing was downloaded. Checking the counters against `netstat` proved the measurement right and the test wrong.

### Megabits, not megabytes

ISPs sell speed in megabits per second, while the OS counts bytes. The dashboard multiplies by 8, so 3 MB/s shows as about 25 Mbps.

### Report = heartbeat (and why agents buffer)

M1 had a separate `Heartbeat` call. Now `Report` carries the samples *and* proves the agent is alive, so one call does both jobs.

If a report fails, its samples go back into the agent's buffer. The buffer holds about 10 minutes and drops the *oldest* samples first, since recent data matters most. A test kills and replaces the hub mid-run and checks that no sample is lost.

### Clock skew: two computers never agree on the time

M4 lines up "the Xbox lagged at 21:04:07" with "Dad's laptop was uploading at 21:04:07". That only works if both timestamps use the same clock. Every report carries the agent's `sent_at`, and the hub computes `skew = hub_now − sent_at` and shifts every sample in that report by the skew. Network delay sneaks into the skew too, but on a LAN that's milliseconds, while unsynced clocks can be off by seconds or minutes. The hub logs agents more than 2 s off.

### Time-series storage: raw → rollups → retention

| Table | Resolution | Kept |
|---|---|---|
| `samples` | 1 s (raw) | 24 h |
| `samples_1m` | 1 minute | 30 days |
| `samples_1h` | 1 hour | 1 year (`-keep-history`) |

- Once a minute, the hub folds finished minutes into `samples_1m` and finished hours into `samples_1h`, then deletes expired rows.
- Rollups store `n`, `n_missing`, `sum`, `min` and `max`, not just the average. That way hours can be built from minutes exactly, and loss (`n_missing`) survives aggregation.
- Lost pings are stored as SQL `NULL`. Aggregate functions skip `NULL`, so averages count real measurements only.
- Each pass recomputes from 15 minutes before the newest rollup. Samples an agent delivers late, after the hub was unreachable, still land in the right minute, and running the job twice changes nothing (it's *idempotent*).
- `Query` picks the table from how far back you ask: raw within 24 h, minutes within 30 days, hours beyond that.

Gotcha found by a test: SQLite stores milliseconds, but Go timestamps carry nanoseconds. A live sample and the same sample read back from the database compared as *different*, so the stream sent duplicates. `Append` now rounds to milliseconds before storing *and* publishing.

### Architecture: reuse over copy-paste

Latency and throughput streams are the same pattern: subscribe, send history, then send live points without duplicates. That logic lives once in the hub as a generic `watch[P]` function, and once on the client as `useLiveStream`. Each RPC only supplies how to turn a stored sample into its own point type. One `TimeSeriesChart` component draws every chart, and one time-window picker at the top of the page drives all of them.

Colors follow the *device*, not its rank: slots are assigned by sorted agent ID, so Dad's laptop doesn't change color when another computer comes online.

## Try it

- Run an upload (`head -c 30000000 /dev/zero | curl -X POST --data-binary @- https://speed.cloudflare.com/__up`) and watch the upload line jump. Watch the latency chart at the same time: the ISP and internet lines rise a little while the router line stays flat. That's the effect M4 will attribute automatically.
- Stop the hub for a minute, start it again, and check that the gap fills in from the agent's buffer.
