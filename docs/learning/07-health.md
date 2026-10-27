# Wi-Fi signal, outages and per-device ping

## What got built

- **Wi-Fi signal.** Each agent reports its computer's signal strength, noise, band (2.4, 5 or 6 GHz), channel and link speed every 30 seconds.
- **Outages and uptime.** An incident is now flagged as an outage when the internet stops answering entirely. Uptime is reported over the last day, week or month.
- **Per-device ping charts.** On the Devices page, any device that answers ping can show its last hour next to the router's.

## Concepts

### dBm: Wi-Fi signal is a negative number

Wi-Fi signal is measured in dBm: decibels relative to one milliwatt. Received Wi-Fi power is tiny, so the number is negative, and closer to zero is stronger. The scale is logarithmic: every 3 dB is half or double the power. Rules of thumb:

| Signal | Means |
|---|---|
| −50 dBm or better | Excellent |
| −67 dBm | The usual minimum for smooth calls and gaming |
| −75 dBm | Weak: lag, retries, dropouts |

Each OS reports this differently, all without admin rights:

- **macOS:** Apple's CoreWLAN framework (`CWWiFiClient`), called from Go through cgo and a few lines of Objective-C. It reads the current connection's numbers from the driver. My Mac: **−47 dBm on 5 GHz channel 153, 866 Mbps link**, which is excellent. Builds without cgo report no Wi-Fi, for the reason below.
- **Windows:** `netsh wlan show interfaces` reports a percentage, which Microsoft maps linearly from −100 dBm (0%) to −50 dBm (100%). The labels are localized, so the parser accepts English and French ("Canal", "Bande", "2,4 GHz"), with tests for both.
- **Linux:** `/proc/net/wireless`.

### The observer effect: measuring Wi-Fi caused lag

The first macOS version used `system_profiler SPAirPortDataType` every 30 seconds, and it reported my signal as a "fair" −68 dBm. Recording the README demo, every lag incident turned out to arrive **exactly 30 seconds apart**, starting 29 seconds after the agent started. That's not Wi-Fi noise; that's a timer.

`system_profiler` also lists *nearby* networks, so each call triggers a Wi-Fi scan, and while scanning, the radio leaves your network's channel for a moment. A direct test pinging the router 5 times a second:

| Router ping | Average | Worst | Over 50 ms |
|---|---|---|---|
| While `system_profiler` runs | 21.9 ms | 138 ms | 9 |
| Nothing running | 5.5 ms | 8 ms | 0 |
| Reading CoreWLAN once a second | 6.4 ms | 16 ms | 0 |

So NetWatch was causing the spikes it then reported as lag, and the "fair" reading was probably taken mid-scan. CoreWLAN reads the current link without scanning. The rule now sits in the package comment: whatever reads Wi-Fi must never scan.

Two lessons: a monitor has to be checked for its own effect on what it measures, and a suspiciously regular pattern is a clue worth chasing.

### A drop is not a slowdown

"Is my internet dropping?" is a different question from "is it slow?". An incident becomes an **outage** when 5 or more internet pings in a row get no answer. Scattered loss stays plain lag, and a test pins down the difference. If the router stopped answering too, the Wi-Fi dropped or the router restarted. If the router kept answering, the provider's line went down.

### Measuring uptime honestly

Uptime = 1 − downtime ÷ time watched. "Time watched" is the trap: if the hub was off for a night, that night isn't uptime *or* downtime, NetWatch simply doesn't know. So the hub counts its own internet pings (one per second) in the window, and only that time goes into the ratio.

That counting surfaced a subtle bug. A "last 24 hours" query is a hair more than 24 hours old by the time it runs, so the store decided it was older than raw retention and read the per-minute rollups. Those don't include the current, unfinished minute, so the count came up short. The table choice now has a few minutes of slack, plus a regression test.

### Per-device ping: the router is the baseline

A device's LAN ping alone is hard to judge. Next to the router's, it's obvious: the router answers in about 2 ms, so any gap between the two lines is the device's own Wi-Fi link. On my network the Xbox has swung from 13 ms to spikes near 190 ms while the router stayed flat. That's its wireless link, not the internet.

### Database upgrades

Adding the `outage` column broke nothing for existing installs. On startup the incidents store checks `pragma_table_info` and runs `ALTER TABLE … ADD COLUMN` if the column is missing. It was tested against a real database created before the change, and a unit test builds the old schema by hand.
