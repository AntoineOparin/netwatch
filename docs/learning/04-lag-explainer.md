# M4: Lag explainer

## What got built

- The hub watches internet ping for **lag incidents**: stretches clearly slower than normal, or with lost pings.
- For each incident it decides whether the router was slow too, meaning Wi-Fi or the local network, or only the connection beyond it.
- It names the computers that were busy at the same moment.
- The dashboard explains each incident in a sentence and shades it on every chart.

## Concepts

### What counts as "lag": baseline, smoothing, hysteresis

- **Baseline:** the *median* internet ping of the last 10 minutes. A median ignores the occasional spike that would drag an average up. No baseline exists until 30 pings have been seen, so there's no "lag" right after startup.
- **Start:** the median of the last 3 pings is more than `-lag-rise` ms (default 30) above baseline, or 2 of the last 5 pings were lost. One stray spike never qualifies: 2 of 3 pings have to be slow.
- **End:** after 5 calm seconds in a row. Using different rules to start and to end (hysteresis) stops one bumpy stretch from turning into five incidents.
- While an incident is open, it's compared against the baseline from *before* it started, so a long incident can't raise its own baseline.
- The reported start and end are the first and last ping that was *itself* slow. The smoothing only decides when the incident is over. My first version used the smoothed times, which added a quiet second at each end and diluted the suspect's numbers. A test caught it.

### Local or upstream?

If the router's ping (median during the incident) was also well above *its* baseline, or 20% of its pings were lost, the incident is **local**: Wi-Fi or the home network. Otherwise it's **upstream**, and that's where "someone is using all the bandwidth" becomes the likely story.

### Suspects: correlation, honestly labeled

- A computer is a suspect if its peak upload passed 1 Mbps, or its peak download passed 5 Mbps, from 3 seconds before the incident until its end. The lead-in catches an upload that starts just before the queue fills.
- Suspects are ranked by upload, because on cable internet the upload is the narrow pipe.
- **Peaks, not averages.** On a real test, the lag came from the first 3 seconds of an upload. An average over that window plus the lead-in reported 3.9 Mbps for an upload that really ran at 15–20 Mbps.
- Agent reports arrive about 2 seconds after the pings, so suspects are only finalized 5 seconds after an incident ends.
- The dashboard says it plainly: "busy at the same moment is a strong hint, not proof". When no computer was busy, it says so and points at devices without an agent, like the Xbox, phones and TVs, or at the ISP.

### What my connection actually does under load

Saturating my upload (about 20 Mbps on Fizz) on purpose, three runs:

1. **TCP slow-start overshoot:** for the first 2–3 seconds, ping jumped from 20 ms to 150–186 ms, because TCP sends faster than the link until it detects the queue. Detected as an incident with the default threshold.
2. **Then the modem copes.** For the rest of the upload, ping stayed at only +15–30 ms. DOCSIS 3.1 modems run an *active queue manager* (PIE) that drops packets early instead of letting the queue grow to hundreds of milliseconds. That's why classic bufferbloat (300 ms+) didn't happen.
3. With `-lag-rise 15` (about where competitive gaming starts to feel it), the whole upload became a 13 s incident: 81 ms peak against a normal 19 ms, blamed on the Mac uploading up to 23.5 Mbps. That run is the one in the screenshot.

So on this connection, someone maxing out the upload costs the Xbox roughly 20–30 ms of extra ping plus a short spike when the upload starts. That's mildly noticeable, not game-breaking. Big lag would point at Wi-Fi (local) or the ISP.

### Architecture

- The `Detector` is **pure**: it's fed samples and uses only their timestamps, never the clock. Tests replay scripted scenarios exactly: an upload causing upstream lag, a slow router meaning Wi-Fi, a single spike that must *not* count, packet loss, and lag before a baseline exists.
- The hub runs it as one more subscriber to the sample stream. It saves each change to an `incidents` table, which keeps `updated_ms`, so an incident cut short by a hub restart gets closed at its last update.
- Incidents are shaded on every chart with one `highlights` prop on the shared chart component.
