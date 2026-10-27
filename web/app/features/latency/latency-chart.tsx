import { useEffect, useMemo, useRef, useSyncExternalStore } from "react";
import type uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import {
  type LatencyData,
  TARGET_COLORS,
  TARGET_LABELS,
  TARGETS,
  type Target,
} from "./stats";

interface Props {
  data: LatencyData;
  windowMs: number;
}

// uPlot wants column arrays sharing one x axis, so samples are bucketed to
// the second they were taken in. Lost pings become gaps in the line plus a
// red tick along the bottom.
interface Aligned {
  targets: Target[];
  columns: uPlot.AlignedData;
  lost: number[]; // x positions (seconds) with at least one lost ping
}

function align(data: LatencyData): Aligned {
  const targets = TARGETS.filter((t) => data[t]?.samples.length);
  const seconds = new Set<number>();
  const byTarget = new Map<Target, Map<number, number | null>>();
  const lost = new Set<number>();
  for (const t of targets) {
    const values = new Map<number, number | null>();
    for (const s of data[t]?.samples ?? []) {
      const sec = Math.floor(s.at / 1000);
      seconds.add(sec);
      values.set(sec, s.rtt);
      if (s.rtt === null) lost.add(sec);
    }
    byTarget.set(t, values);
  }
  const xs = [...seconds].sort((a, b) => a - b);
  return {
    targets,
    columns: [
      xs,
      ...targets.map((t) => xs.map((x) => byTarget.get(t)?.get(x) ?? null)),
    ],
    lost: [...lost],
  };
}

const darkQuery = "(prefers-color-scheme: dark)";
function useColorScheme() {
  return useSyncExternalStore(
    (onChange) => {
      const mq = window.matchMedia(darkQuery);
      mq.addEventListener("change", onChange);
      return () => mq.removeEventListener("change", onChange);
    },
    () => (window.matchMedia(darkQuery).matches ? "dark" : "light"),
    () => "light",
  );
}

// Canvas can't use var(--x) directly, so resolve the CSS variable first.
function cssVar(value: string) {
  const name = value.match(/var\((--[\w-]+)\)/)?.[1];
  return name
    ? getComputedStyle(document.documentElement).getPropertyValue(name).trim()
    : value;
}

const HEIGHT = 260;

export function LatencyChart({ data, windowMs }: Props) {
  const container = useRef<HTMLDivElement>(null);
  const plot = useRef<uPlot | null>(null);
  const aligned = useMemo(() => align(data), [data]);
  const latest = useRef(aligned);
  latest.current = aligned;
  const scheme = useColorScheme();
  // Rebuild the plot when the set of series changes (e.g. ISP hop appears).
  const seriesKey = aligned.targets.join(",");

  // biome-ignore lint/correctness/useExhaustiveDependencies: scheme and seriesKey are rebuild triggers; the plot reads colors and series via refs.
  useEffect(() => {
    const el = container.current;
    if (!el) return;
    let cancelled = false;
    let observer: ResizeObserver | undefined;

    // Loaded lazily: uPlot touches `window`, and the SPA build pre-renders
    // index.html in Node.
    import("uplot").then(({ default: UPlot }) => {
      if (cancelled) return;
      const targets = latest.current.targets;
      const axis = {
        stroke: cssVar("var(--chart-axis)"),
        grid: { stroke: cssVar("var(--chart-grid)"), width: 1 },
        ticks: { show: false },
      };
      const critical = cssVar("var(--status-critical)");
      const windowSec = windowMs / 1000;

      plot.current = new UPlot(
        {
          width: el.clientWidth,
          height: HEIGHT,
          legend: { show: false },
          cursor: { y: false, points: { size: 8 } },
          scales: {
            x: { time: true, range: (_u, _min, max) => [max - windowSec, max] },
            y: { range: (_u, _min, max) => [0, Math.max(50, max * 1.15)] },
          },
          axes: [
            axis,
            {
              ...axis,
              size: 56,
              values: (_u, splits) => splits.map((v) => `${v} ms`),
            },
          ],
          series: [
            {},
            ...targets.map((t) => ({
              label: TARGET_LABELS[t],
              stroke: cssVar(TARGET_COLORS[t]),
              width: 2,
              points: { show: false },
            })),
          ],
          hooks: {
            draw: [(u) => drawLost(u, latest.current.lost, critical)],
          },
          plugins: [tooltipPlugin(targets)],
        },
        latest.current.columns,
        el,
      );
      observer = new ResizeObserver(() =>
        plot.current?.setSize({ width: el.clientWidth, height: HEIGHT }),
      );
      observer.observe(el);
    });

    return () => {
      cancelled = true;
      observer?.disconnect();
      plot.current?.destroy();
      plot.current = null;
    };
  }, [scheme, seriesKey, windowMs]);

  useEffect(() => {
    plot.current?.setData(aligned.columns);
  }, [aligned]);

  return <div ref={container} className="relative w-full" />;
}

function drawLost(u: uPlot, lost: number[], color: string) {
  const { ctx } = u;
  const { left, top, width, height } = u.bbox;
  const tick = 8 * devicePixelRatio;
  ctx.save();
  ctx.fillStyle = color;
  for (const sec of lost) {
    const x = u.valToPos(sec, "x", true);
    if (x < left || x > left + width) continue;
    ctx.fillRect(
      x - devicePixelRatio,
      top + height - tick,
      2 * devicePixelRatio,
      tick,
    );
  }
  ctx.restore();
}

// One tooltip listing every hop at the hovered second, value first. Text is
// set with textContent so nothing from the data is parsed as HTML.
function tooltipPlugin(targets: Target[]): uPlot.Plugin {
  let tip: HTMLDivElement;
  return {
    hooks: {
      init: (u) => {
        tip = document.createElement("div");
        tip.className =
          "pointer-events-none absolute z-10 hidden min-w-40 rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs shadow-sm dark:border-gray-700 dark:bg-gray-900";
        u.over.appendChild(tip);
      },
      setCursor: (u) => {
        const idx = u.cursor.idx;
        if (idx == null || u.cursor.left == null || u.cursor.left < 0) {
          tip.classList.add("hidden");
          return;
        }
        const time = document.createElement("div");
        time.className = "mb-1 text-gray-500";
        time.textContent = new Date(
          (u.data[0][idx] ?? 0) * 1000,
        ).toLocaleTimeString();
        const rows = targets.map((t, i) => {
          const v = u.data[i + 1][idx];
          const row = document.createElement("div");
          row.className = "flex items-center gap-2 py-0.5";
          const key = document.createElement("span");
          key.className = "h-0.5 w-3 rounded-full";
          key.style.background = TARGET_COLORS[t];
          const value = document.createElement("span");
          value.className = "w-14 font-semibold tabular-nums";
          value.textContent = v == null ? "lost" : `${v.toFixed(1)} ms`;
          const label = document.createElement("span");
          label.className = "text-gray-500";
          label.textContent = TARGET_LABELS[t];
          row.append(key, value, label);
          return row;
        });
        tip.replaceChildren(time, ...rows);
        tip.classList.remove("hidden");
        // Flip to the left of the cursor near the right edge.
        const left = u.cursor.left;
        const flip = left > u.over.clientWidth - 180;
        tip.style.left = `${flip ? left - tip.offsetWidth - 12 : left + 12}px`;
        tip.style.top = "8px";
      },
    },
  };
}
