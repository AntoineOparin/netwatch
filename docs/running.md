# Running NetWatch at home

NetWatch has two programs:

| Program | Runs on | Does |
|---|---|---|
| **hub** | One always-on-ish machine (a Mac, a Linux box, a Raspberry Pi) | Pings the router, the ISP and the internet; finds every device on the network; stores everything; serves the dashboard at `http://<hub-ip>:8080` |
| **agent** | Every computer you want per-computer numbers for | Measures that computer's download/upload (and, on macOS, which apps) and reports it to the hub every 2 s |

The hub alone already answers "is it the Wi-Fi, the internet, or my ISP?" and lists every device. Agents add "which computer, and which app".

## 1. Get the binaries

Either download them from the repo's **Releases** page (`netwatch-hub-…` and `netwatch-agent-…` for your OS and CPU), or build from source:

```sh
cd web && bun install && cd ..
make            # bin/hub (dashboard included) and bin/agent for this machine
make release    # every platform, in dist/
```

Which file to pick:

| Computer | Agent file |
|---|---|
| Mac with Apple silicon (M1 and later) | `netwatch-agent-darwin-arm64` |
| Older Intel Mac | `netwatch-agent-darwin-amd64` |
| Windows PC | `netwatch-agent-windows-amd64.exe` |
| Linux / Raspberry Pi 4–5 | `netwatch-agent-linux-amd64` / `-linux-arm64` |

## 2. Start the hub

```sh
./netwatch-hub
```

It prints its address and stores data in `./netwatch.db`. Find the machine's LAN IP (macOS: System Settings → Wi-Fi → Details; or `ipconfig getifaddr en0`), then open `http://<that-ip>:8080` from any device on the network, including your phone.

- **macOS firewall:** if it's on, macOS asks whether `netwatch-hub` may accept incoming connections. Allow it, or other computers' agents can't report.
- **Linux:** unprivileged ping needs `net.ipv4.ping_group_range` to include your group (it does by default on most current distros). Otherwise run with `-probe-privileged` as root.
- Useful flags: `-lag-rise 15` (more sensitive lag detection), `-sweep-every 0` (no device discovery), `-db /path/netwatch.db`.

## 3. Start an agent on each computer

```sh
./netwatch-agent -hub http://192.168.0.155:8080      # macOS / Linux
netwatch-agent-windows-amd64.exe -hub http://192.168.0.155:8080   # Windows (Command Prompt)
```

Use the hub's IP, not `localhost`, unless the agent runs on the hub machine itself. The computer shows up on the dashboard within a few seconds.

**Unsigned binary warnings.** The release binaries aren't code-signed:

- macOS says it "cannot be opened because the developer cannot be verified". Run `xattr -d com.apple.quarantine netwatch-agent-darwin-arm64` once, or right-click → Open.
- Windows SmartScreen says "Windows protected your PC". Click "More info" → "Run anyway".

**Work computers:** check before installing anything on an employer's laptop. It's their device and their policy.

## How much disk it uses

Everything lives in one SQLite file (`netwatch.db`), and every kind of data has a ceiling. Once a minute the hub folds raw readings into averages and deletes what has expired:

| What | Kept for |
|---|---|
| Raw readings (every ping, every speed sample) | 24 hours |
| Per-minute averages | 30 days |
| Per-hour averages, lag incidents and outages | 1 year (`-keep-history`, e.g. `-keep-history 17520h` for 2 years) |
| Devices not seen on the network | Forgotten after 90 days (`-forget-devices`); a device that comes back starts fresh, without its nickname |

For a typical home (the hub, a couple of computers running the agent, a dozen devices) the file levels off at roughly 100 MB. SQLite reuses the space freed by deletions, so it doesn't keep growing.

## Start automatically

### macOS (hub or agent) with launchd

Save as `~/Library/LaunchAgents/dev.netwatch.agent.plist`, with your paths:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>dev.netwatch.agent</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Users/you/netwatch/netwatch-agent-darwin-arm64</string>
    <string>-hub</string><string>http://192.168.0.155:8080</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardErrorPath</key><string>/tmp/netwatch-agent.log</string>
</dict>
</plist>
```

Then `launchctl load ~/Library/LaunchAgents/dev.netwatch.agent.plist`. For the hub, do the same with the hub binary, and add `-db` with a full path so the database doesn't depend on the working directory.

### Windows (agent) with Task Scheduler

In Command Prompt, with your paths:

```bat
schtasks /Create /SC ONLOGON /TN "NetWatch agent" /TR "\"C:\Users\you\netwatch\netwatch-agent-windows-amd64.exe\" -hub http://192.168.0.155:8080"
```

A console window opens at log-in; minimize it. `schtasks /Delete /TN "NetWatch agent"` removes it.

## What each OS can measure

| | macOS | Windows | Linux |
|---|---|---|---|
| Agent: computer's download/upload | ✓ | ✓ | ✓ |
| Agent: per-app breakdown | ✓ (`nettop`) | needs admin tracing (ETW), not built yet | needs eBPF, not built yet |
| Hub | ✓ | not supported (no unprivileged ping) | ✓ |
