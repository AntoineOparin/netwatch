# M2: Lag probe

## What got built

- The hub pings three hops once a second: my router, the first ISP router and 1.1.1.1. It records round-trip time and loss.
- The dashboard streams the results live, shows average, jitter and loss over the last minute, and gives a plain-language verdict on where any lag starts.

## Concepts

### ICMP echo: what "ping" actually is

A ping is an ICMP *echo request*. The target answers with an *echo reply* carrying the same ID and sequence number, and the time between the two is the round-trip time (RTT). No reply within 1 s counts as a lost packet.

The pinger (`internal/collector/probe/icmp.go`) is written by hand with `golang.org/x/net/icmp` rather than a ping library, to see the bytes. Things that surprised me:

- **No root needed on macOS.** A datagram ICMP socket (`"udp4"`) is allowed for normal users. Linux allows it only when `net.ipv4.ping_group_range` includes your group. Otherwise use `-probe-privileged`, which opens a raw socket (`"ip4:icmp"`) and needs root or `CAP_NET_RAW`.
- **Linux rewrites the echo ID** on datagram sockets, so replies are matched by sender address and sequence number only.
- **One socket can see other replies**, such as a late reply to an earlier ping. The read loop throws away anything that isn't the reply it's waiting for.

### TTL and how traceroute works

Every IP packet carries a TTL (time to live). Each router decrements it, and the router that takes it to 0 drops the packet and sends back ICMP *time exceeded*, from its own address. Sending pings toward 1.1.1.1 with TTL 1, 2, 3, … reveals each router in turn. On my network:

| TTL | Router | Who |
|---|---|---|
| 1 | 192.168.0.1 | my Fizz router (CODA-4680) |
| 2 | 24.225.244.1 | Fizz's network edge, used as the "ISP" target |
| 3 | 10.154.99.122 | ISP core (a private address inside their network) |
| 5 | 162.158.124.25 | Cloudflare |
| 6 | 1.1.1.1 | echo reply: arrived |

Hop 4 never answered, which is normal: some routers don't send time-exceeded messages at all. To check that a time-exceeded message belongs to our probe, `quotedEchoSeq` parses the copy of our original packet that the router quotes back: the IP header, then our echo's sequence number.

### Reading the three lines

Each hop includes every hop before it, so the first hop that goes bad is where the problem is:

- **Router bad** means Wi-Fi or the local network. Everything after it inherits the problem.
- **Router fine, ISP bad** means the link between the house and the ISP is congested. The usual cause is *bufferbloat*: someone maxes out the upload, the modem's queue fills, and every packet, including the Xbox's, waits in line. M4 will name who.
- **Only the internet target is bad** means the problem is further out, not in the house.

Caveat: routers answer pings aimed *at themselves* at low priority, so the ISP hop can look noisier than the internet target. The verdict uses looser thresholds for it.

### Jitter matters more than ping for games

Jitter is the average change between consecutive RTTs. A steady 40 ms feels fine in a game, but 20 ms jumping to 120 ms feels like lag. Real finding from the first run: **my laptop's Wi-Fi alone adds about 15 ms of jitter**, and one spike pushed all three hops to 160 ms at the same moment. Since the router spiked too, that was Wi-Fi, not the internet.

### Architecture: the collector seam and streaming

- The probe is the first `collector.Collector`. `collector.RunAll` restarts it if it fails, for example when the network changes and the gateway disappears.
- Samples go into `series.Store`: in memory, 15 minutes of retention, and fan-out to subscribers. A lost ping is stored as `NaN` so that loss and latency share one series. The API turns `NaN` into `lost: true`.
- The dashboard uses a **server-streaming RPC** (`WatchLatency`) instead of polling. History arrives first, then each new point. The handler subscribes *before* it reads history and skips duplicates, so no point is lost or sent twice in between.
