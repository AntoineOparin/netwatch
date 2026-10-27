# M5: Every device on the LAN

## What got built

- The hub finds every device on the home network, including those that can't run an agent: the Xbox, phones, the TV.
- It names them, looks up the maker, and pings the ones that answer to rate their Wi-Fi.
- The Devices page lists them all and lets you rename them ("Xbox").

## Concepts

### Layer 2 vs layer 3: why ARP beats ping for finding devices

First sweep of my network: **3 devices answered ping, but 7 were in the ARP table.**

- **Ping** is ICMP, layer 3. A device can ignore it: phones in power saving, laptops with a firewall, many consoles.
- **ARP** is layer 2. Before my Mac can send anything to `192.168.0.42`, it broadcasts "who has 192.168.0.42?" and the owner answers with its MAC address. A device that ignored ARP couldn't receive anything at all, so everything on the network answers.

So the sweep pings every address (`192.168.0.1`–`.254`), but only to make the OS ARP for each one. Then it reads the OS's ARP table (`arp -an` on macOS, `/proc/net/arp` on Linux, `arp -a` on Windows), and anything listed is present. The three parsers are pure functions with tests, so all three OS formats are tested on any machine.

Caveat: macOS keeps ARP entries for up to 20 minutes, so a device that leaves can still look online for a while. Sending raw ARP requests ourselves would fix that, but needs root (`gopacket`), which is a good job for the Raspberry Pi later.

### Identity: MAC addresses, and why most of mine are "private"

Devices are keyed by MAC, because the IP can change every time DHCP hands out a lease. The first 3 bytes of a MAC are the maker's ID (OUI), looked up in the IEEE registry: `b4:f7:a1` is LG and `74:70:fd` is Intel.

But **5 of my 8 devices have made-up MACs**, including my Mac, both iPhones/iPads and even the router's LAN side. If the second-lowest bit of the first byte is set, the address is *locally administered*: modern phones, tablets and laptops invent a random MAC per Wi-Fi network, so they can't be tracked from network to network. The maker of those devices is unknowable, and the dashboard says "private address, maker hidden" instead of guessing.

### Names: ask the router, not "the DNS"

Home routers register the name each device sends when it asks for an IP (DHCP), so reverse DNS for `192.168.0.167` returns `iPhone.hitronhub.home`. My first attempt found **no names at all**: Go asked the Mac's configured DNS servers, which are Fizz's IPv6 resolvers on the internet, and those know nothing about my LAN. Discovery now sends reverse lookups straight to the router's DNS on port 53, with the system resolver as fallback.

### LAN ping: Wi-Fi quality, and phones that doze

Devices that answer ping get pinged every 5 seconds, keeping 10 minutes of history. Readings from my network:

- **Router:** 2 ms, ±2 ms. That's what good Wi-Fi looks like.
- **iPhone:** 177 ms, ±235 ms jitter, 11% lost. **LG phone:** 251 ms.

The phones aren't on terrible Wi-Fi. They're **saving battery**: in Wi-Fi power-save mode, a phone tells the router "hold my packets", sleeps, and only wakes every ~100–300 ms to collect them. A phone's LAN ping says little about its Wi-Fi. A console's or PC's says a lot, since they stay awake. That's the number to watch for the Xbox: if it's slow or jittery while the router is fast, the Xbox's own Wi-Fi link is the weak spot.

### Architecture

- The ICMP pinger moved into its own package (`internal/icmp`), now shared by the probe and discovery. It gained `Sweep`: send to every address over one socket, then collect replies until the timeout.
- Discovery is another `Collector`. Round trips flow through the normal sample pipeline (`lan_rtt_ms`, keyed by MAC). The *inventory* (who exists, their name, maker and IP) isn't a time series, so it goes through a small `Inventory` interface instead. The SQLite `devices` store implements it, and discovery doesn't know SQL exists.
- The device table preserves what shouldn't be lost: your nickname, the first-seen time, and a known hostname when the router's DNS briefly has no answer.
- Agents and devices are linked by IP, so "this computer runs the agent" shows up on the device list.
- `hub.NewServer` had grown to six positional arguments, so it now takes one `Config` struct.
