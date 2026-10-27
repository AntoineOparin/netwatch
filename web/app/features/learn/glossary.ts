// Plain-language explanations of every term the dashboard uses. Info tips
// show `short`; the Learn page shows both.

export interface Term {
  term: string;
  short: string;
  more?: string;
}

export const GLOSSARY = {
  ping: {
    term: "Ping (round-trip time)",
    short:
      "How long a tiny message takes to reach a device and come back, in milliseconds (ms). Lower is better: under 30 ms feels instant in games, over 100 ms feels laggy.",
    more: "The hub pings your router, your internet provider and a server on the internet once a second, so it notices slowdowns the moment they start.",
  },
  jitter: {
    term: "Jitter",
    short:
      "How much the ping jumps around from one second to the next. A steady 40 ms is fine for gaming; 20 ms jumping to 120 ms feels like lag even though the average looks OK.",
  },
  loss: {
    term: "Packet loss (lost pings)",
    short:
      "Messages that never came back. Even 1–2% loss causes stutters and rubber-banding in games and video calls.",
  },
  router: {
    term: "Your router",
    short:
      "The box from your internet provider that every device connects to (by Wi-Fi or cable). It's the first stop for everything leaving your home.",
    more: "If even the router is slow to answer, the problem is inside your home: usually Wi-Fi signal or interference.",
  },
  isp: {
    term: "ISP (first hop)",
    short:
      "Your internet service provider's first router, just outside your home. If your router is fast but this is slow, your connection to the provider is congested or the provider is having trouble.",
  },
  internet: {
    term: "Internet target",
    short:
      "A fast, always-on server on the internet (1.1.1.1, run by Cloudflare). Pinging it shows what your whole connection feels like end to end.",
  },
  hops: {
    term: "The three hops",
    short:
      "Your data travels device → router → internet provider → internet. Pinging each stop shows where a slowdown starts: the first slow stop is where the trouble is.",
  },
  mbps: {
    term: "Mbps (megabits per second)",
    short:
      "How fast data is moving. Internet plans are sold in Mbps. Video calls use about 2–4 Mbps, 4K streaming about 25 Mbps, a big download as much as your plan allows.",
  },
  updown: {
    term: "Download vs upload",
    short:
      "Download is data coming into your home (streaming, browsing). Upload is data going out (video calls, cloud backups, sending files). Home connections usually have far less upload than download.",
  },
  bufferbloat: {
    term: "Bufferbloat",
    short:
      "When someone maxes out the connection (often a big upload or backup), packets queue up in the modem and everyone's ping rises, including the Xbox's, even though nothing is broken.",
    more: "It's why one person's cloud backup can make a game lag for someone else. Modern modems manage the queue to limit it; NetWatch measures how much it actually happens on your connection.",
  },
  incident: {
    term: "Lag incident",
    short:
      "A stretch of time when the internet ping was clearly higher than normal (30 ms or more above usual) or pings were being lost. NetWatch detects these automatically.",
  },
  baseline: {
    term: "Normal (baseline) ping",
    short:
      "What your internet ping usually is, measured as the middle value over the last 10 minutes. A lag incident is judged against this, so a connection that's always 40 ms isn't flagged for being 40 ms.",
  },
  suspect: {
    term: "Busy at the same time",
    short:
      "Computers (and apps) that were using a lot of bandwidth while the lag happened. A strong hint, not proof: something else, like the Xbox itself or a phone, could also be the cause.",
  },
  hub: {
    term: "Hub",
    short:
      "The main NetWatch program. It runs on one computer, does all the measuring of your connection and devices, stores the history and shows this dashboard.",
  },
  agent: {
    term: "Agent",
    short:
      "A small helper program you run on each computer you want to measure. It reports that computer's download and upload (and, on Macs, which apps) to the hub every 2 seconds.",
    more: "Consoles, phones and TVs can't run an agent, so NetWatch can see them on the network but can't measure their traffic.",
  },
  wifiSignal: {
    term: "Wi-Fi signal (dBm)",
    short:
      "How strong a computer's Wi-Fi connection is. It's a negative number: closer to zero is stronger. Around −50 is excellent, −67 is good enough for games and calls, below −75 is weak and causes lag and drops.",
    more: "5 GHz Wi-Fi is faster but has shorter range than 2.4 GHz. Moving closer to the router, or using a cable, fixes a weak signal.",
  },
  outage: {
    term: "Outage",
    short:
      "A time when the internet stopped answering completely (several pings lost in a row), rather than just getting slow. If your router stopped answering too, it was the Wi-Fi or the router itself.",
  },
  uptime: {
    term: "Uptime",
    short:
      "The share of time the internet was working, while NetWatch was watching. 99.9% means about a minute and a half of outages per day.",
  },
  lan: {
    term: "LAN (local network)",
    short:
      "Your home network: every device connected to your router, by Wi-Fi or cable.",
  },
  ip: {
    term: "IP address",
    short:
      "A device's address on the network, like 192.168.0.42. The router hands these out and they can change over time.",
  },
  mac: {
    term: "MAC address",
    short:
      "A device's hardware ID, like 7c:ed:8d:01:02:03. Its first half usually tells you the maker (Microsoft, Apple, LG…). NetWatch uses it to recognize a device even when its IP address changes.",
  },
  privateMac: {
    term: "Private address",
    short:
      "Phones, tablets and many laptops make up a random MAC address for each Wi-Fi network, for privacy. That means their maker can't be looked up; their name usually comes from the router instead.",
  },
  arp: {
    term: "ARP (how devices are found)",
    short:
      'Before talking to a device on your network, a computer asks "who has this address?" and the device answers. Every device must answer, so NetWatch finds even ones that ignore pings.',
  },
  lanPing: {
    term: "LAN ping",
    short:
      "Ping from the hub to a device on your network. A few ms means a good connection. A console or PC that's slow here has weak Wi-Fi.",
    more: "Phones often look slow because they doze between Wi-Fi check-ins to save battery; that's normal and doesn't affect what they're doing.",
  },
} satisfies Record<string, Term>;

export type TermId = keyof typeof GLOSSARY;
