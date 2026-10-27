# M6: Which app is using the bandwidth

## What got built

- On macOS, the agent reports which **apps** are using the network, not just the computer's total.
- The dashboard lists the busiest apps under each computer ("curl ↑ 4.9 Mbps").
- Incident explanations name the app: "Dads-Laptop was uploading up to 9.4 Mbps (mostly OneDrive)".

## Concepts

### Where per-process numbers come from (and why it's OS-specific)

The OS counts bytes per *interface* for free (that's M3). Counting per *process* means attributing every packet to the socket, and then the process, that sent it. That attribution happens deep in the kernel, so every OS exposes it differently:

| OS | How | Needs root? |
|---|---|---|
| macOS | `nettop` (Apple's NetworkStatistics framework) | **No** |
| Windows | ETW tracing (`Microsoft-Windows-Kernel-Network`) | Yes (admin) |
| Linux | eBPF, or packet capture + socket→PID mapping (how `nethogs` works) | Yes |

Only macOS is implemented, because it works unprivileged and I can verify it on my machine. On other OSes `procnet.Supported()` is false, the agent skips the collector, and the dashboard says "per-app breakdown is macOS-only for now". It doesn't silently show nothing. The OS-specific code sits behind Go build tags (`procnet_darwin.go` / `procnet_other.go`), while the parsing logic is plain Go and tested on every OS.

### Reading `nettop`

`nettop -P -d -L 0 -s 2 -J bytes_in,bytes_out -x` prints a CSV block every 2 seconds:

```
,bytes_in,bytes_out,
curl.54588,4550119,520,
mDNSResponder.476,596,436,
```

- `-d` (delta mode) means each block holds the bytes moved *during that interval*. The very first block is different: totals since each process started, so it's skipped.
- Process names are cut to 15 characters (`com.apple.WebKi`), and the PID is glued on after a dot. Names contain dots too, so the PID is whatever follows the *last* one.
- To get real names, each PID's executable path is looked up once, and the outermost `.app` bundle is used. All of Chrome's helper processes live inside `Google Chrome.app`, so they add up to one "Google Chrome".
- Each sample keeps the 8 busiest apps above 1 KB/s, which keeps the number of time series sane.

### The bug: buffered output from a pipe

First real test: curl uploaded about 37 MB, but the hub only saw one 2-second slice of it. Watching when each block arrived made the cause obvious:

```
pipe:   header at 43.16, 43.16, 43.16, 43.16   (all at once, when nettop exited)
pty:    header at 44.18, 45.18, 46.18           (one per second)
```

C programs buffer their output in 4–16 KB chunks when writing to a pipe, and flush line by line only when writing to a terminal. So `nettop` now runs under `script -q /dev/null`, which gives it a pseudo-terminal. Side effects to clean up: a `\r` on every line and an echoed `^D` at the start (`cleanLine`, with a test). After the fix, curl showed up correctly: an average of 4.9 Mbps over the minute, which matches 37 MB in 60 s, and a peak of 21.4 Mbps.

### Rates vs volumes

Agents send *rates* every 2 seconds, and only for busy apps. To rank apps over a minute, the hub turns them back into data volume (rate × 2 s per sample, with missing samples counted as zero) and divides by the window. That's why `procnet.Interval` is an exported constant: both sides agree on it. Apps averaging under 0.05 Mbps are hidden as chatter.

### Naming the app behind a lag spike

The incident detector also keeps per-app rates. When it names a suspect computer, it picks that computer's app with the most data in the suspect's busy direction: upload if the computer was uploading, download otherwise. Per-app samples reach the hub later than ping results (2 s sampling + 2 s reporting), so suspects are now finalized 8 s after an incident ends instead of 5.
