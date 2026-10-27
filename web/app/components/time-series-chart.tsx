import {
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import type uPlot from "uplot";
import "uplot/dist/uPlot.min.css";

export interface ChartPoint {
  at: number; // epoch milliseconds
  value: number | null; // null = measured, but no value (lost ping)
}

export interface ChartSeries {
  key: string;
  label: string;
  color: string; // CSS color or var(--x)
  points: ChartPoint[];
}

interface Props {
  series: ChartSeries[];
  windowMs: number;
  formatValue: (v: number) => string;
  // The y axis always reaches at least this high, so a quiet minute doesn't
  // blow tiny wiggles up to full height.
  yFloor: number;
  // Draw a red tick along the bottom wherever a point has no value.
  markMissing?: boolean;
  // Time ranges (epoch ms) to shade behind the lines, e.g. lag incidents.
  highlights?: Highlight[];
  height?: number;
  // Drag across the chart to zoom into that stretch (the y axis rescales to
  // what's visible); double-click or "Reset zoom" to go back. Off for live
  // overview charts, whose window is set by the page.
  zoomable?: boolean;
}

export interface Highlight {
  from: number;
  to: number;
}

const NO_HIGHLIGHTS: Highlight[] = [];

// uPlot wants column arrays sharing one x axis, so points are bucketed to the
// second they were taken in. Missing values become gaps in the line.
interface Aligned {
  columns: uPlot.AlignedData;
  missing: number[]; // x positions (seconds) where any series had no value
}

const GAP_SECONDS = 10;

function align(series: ChartSeries[]): Aligned {
  const seconds = new Set<number>();
  const missing = new Set<number>();
  const byKey = series.map((s) => {
    const values = new Map<number, number | null>();
    for (const p of s.points) {
      const sec = Math.floor(p.at / 1000);
      seconds.add(sec);
      values.set(sec, p.value);
      if (p.value === null) missing.add(sec);
    }
    return values;
  });
  // Where data stops for a while (hub restarted, agent asleep), add an
  // empty slot so the line breaks instead of drawing a straight line
  // through time nothing was measured.
  const xs: number[] = [];
  for (const x of [...seconds].sort((a, b) => a - b)) {
    const prev = xs.at(-1);
    if (prev !== undefined && x - prev > GAP_SECONDS) xs.push(prev + 1);
    xs.push(x);
  }
  return {
    columns: [xs, ...byKey.map((v) => xs.map((x) => v.get(x) ?? null))],
    missing: [...missing],
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

export function TimeSeriesChart({
  series,
  windowMs,
  formatValue,
  yFloor,
  markMissing = false,
  highlights = NO_HIGHLIGHTS,
  height = 240,
  zoomable = false,
}: Props) {
  // The zoomed x range, in seconds. A ref so data refreshes (which re-run
  // auto-ranging) keep the zoom; state only drives the Reset button.
  const zoom = useRef<{ min: number; max: number } | null>(null);
  const [zoomed, setZoomed] = useState(false);
  const resetZoom = () => {
    zoom.current = null;
    setZoomed(false);
    const u = plot.current;
    if (u) u.setData(u.data, true);
  };
  const container = useRef<HTMLDivElement>(null);
  const plot = useRef<uPlot | null>(null);
  const aligned = useMemo(() => align(series), [series]);
  const latest = useRef({ aligned, series, formatValue, highlights });
  latest.current = { aligned, series, formatValue, highlights };
  const scheme = useColorScheme();
  // Rebuild the plot when the set of series changes (an agent appears).
  const seriesKey = series.map((s) => `${s.key}:${s.color}`).join(",");

  // biome-ignore lint/correctness/useExhaustiveDependencies: scheme and seriesKey are rebuild triggers; the plot reads data and series via refs.
  useEffect(() => {
    const el = container.current;
    if (!el) return;
    let cancelled = false;
    let observer: ResizeObserver | undefined;

    // Loaded lazily: uPlot touches `window`, and the SPA build pre-renders
    // index.html in Node.
    import("uplot").then(({ default: UPlot }) => {
      if (cancelled) return;
      const axis = {
        stroke: cssVar("var(--chart-axis)"),
        grid: { stroke: cssVar("var(--chart-grid)"), width: 1 },
        ticks: { show: false },
      };
      const critical = cssVar("var(--status-critical)");
      const warning = cssVar("var(--status-warning)");
      const windowSec = windowMs / 1000;

      plot.current = new UPlot(
        {
          width: el.clientWidth,
          height,
          legend: { show: false },
          cursor: {
            y: false,
            points: { size: 8 },
            // uPlot's own drag-zoom would fight the fixed window, so zooming
            // (when enabled) is handled in the setSelect hook below.
            drag: { x: zoomable, y: false, setScale: false },
          },
          scales: {
            x: {
              time: true,
              range: (_u, _min, max) =>
                zoom.current
                  ? [zoom.current.min, zoom.current.max]
                  : [max - windowSec, max],
            },
            y: {
              range: (_u, _min, max) => [0, Math.max(yFloor, max * 1.15)],
            },
          },
          axes: [
            {
              ...axis,
              // One line per tick: "6:12 PM" on whole minutes, ":30" in
              // between. uPlot's default adds the date to the first tick,
              // which collides with its neighbour.
              values: (_u, splits) => splits.map(timeTick),
            },
            {
              ...axis,
              size: 64,
              values: (_u, splits) =>
                splits.map((v) => latest.current.formatValue(v)),
            },
          ],
          series: [
            {},
            ...latest.current.series.map((s) => ({
              label: s.label,
              stroke: cssVar(s.color),
              width: 2,
              points: { show: false },
            })),
          ],
          hooks: {
            init: [
              (u) => {
                if (zoomable) u.over.addEventListener("dblclick", resetZoom);
              },
            ],
            // A finished drag: zoom into the selected stretch.
            setSelect: [
              (u) => {
                if (!zoomable || u.select.width < 8) return;
                const min = u.posToVal(u.select.left, "x");
                const max = u.posToVal(u.select.left + u.select.width, "x");
                u.setSelect({ left: 0, top: 0, width: 0, height: 0 }, false);
                zoom.current = { min, max };
                setZoomed(true);
                u.setData(u.data, true);
              },
            ],
            // drawClear runs before the series, so bands sit behind lines.
            drawClear: [
              (u) => drawHighlights(u, latest.current.highlights, warning),
            ],
            draw: markMissing
              ? [
                  (u) =>
                    drawMissing(u, latest.current.aligned.missing, critical),
                ]
              : [],
          },
          plugins: [tooltipPlugin(() => latest.current)],
        },
        latest.current.aligned.columns,
        el,
      );
      observer = new ResizeObserver(() =>
        plot.current?.setSize({ width: el.clientWidth, height }),
      );
      observer.observe(el);
    });

    return () => {
      cancelled = true;
      observer?.disconnect();
      plot.current?.destroy();
      plot.current = null;
      // A new window (or a rebuild) starts zoomed out.
      zoom.current = null;
      setZoomed(false);
    };
  }, [scheme, seriesKey, windowMs, yFloor, markMissing, height, zoomable]);

  useEffect(() => {
    plot.current?.setData(aligned.columns);
  }, [aligned]);

  return (
    <div className="relative">
      <div
        ref={container}
        className={`relative w-full ${zoomable ? "cursor-crosshair select-none" : ""}`}
      />
      {zoomed && (
        <button
          type="button"
          onClick={resetZoom}
          className="absolute top-1 right-1 z-10 rounded-md bg-white/90 px-2 py-1 text-xs font-medium text-zinc-700 shadow-sm ring-1 ring-zinc-950/10 backdrop-blur transition hover:bg-zinc-100 hover:text-zinc-900 active:scale-[0.97] dark:bg-zinc-900/90 dark:text-zinc-200 dark:ring-white/10 dark:hover:bg-zinc-800 dark:hover:text-white"
        >
          Reset zoom
        </button>
      )}
    </div>
  );
}

function timeTick(sec: number) {
  const d = new Date(sec * 1000);
  return d.getSeconds() === 0
    ? d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })
    : `:${String(d.getSeconds()).padStart(2, "0")}`;
}

function drawHighlights(u: uPlot, ranges: Highlight[], color: string) {
  const { ctx } = u;
  const { left, top, width, height } = u.bbox;
  ctx.save();
  ctx.fillStyle = color;
  ctx.globalAlpha = 0.18;
  for (const r of ranges) {
    const x0 = Math.max(left, u.valToPos(r.from / 1000, "x", true));
    // At least 2px wide, so a short incident is still visible.
    const x1 = Math.min(left + width, u.valToPos(r.to / 1000, "x", true));
    if (x1 < left || x0 > left + width) continue;
    ctx.fillRect(x0, top, Math.max(x1 - x0, 2 * devicePixelRatio), height);
  }
  ctx.restore();
}

function drawMissing(u: uPlot, missing: number[], color: string) {
  const { ctx } = u;
  const { left, top, width, height } = u.bbox;
  const tick = 8 * devicePixelRatio;
  ctx.save();
  ctx.fillStyle = color;
  for (const sec of missing) {
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

// One tooltip listing every series at the hovered second, value first. Text
// is set with textContent so nothing from the data is parsed as HTML.
function tooltipPlugin(
  current: () => {
    series: ChartSeries[];
    formatValue: (v: number) => string;
  },
): uPlot.Plugin {
  let tip: HTMLDivElement;
  return {
    hooks: {
      init: (u) => {
        tip = document.createElement("div");
        tip.className =
          "pointer-events-none absolute z-10 hidden min-w-40 rounded-lg border border-zinc-200 bg-white px-3 py-2 text-xs shadow-lg shadow-zinc-950/10 dark:border-zinc-700 dark:bg-zinc-900";
        u.over.appendChild(tip);
      },
      setCursor: (u) => {
        const idx = u.cursor.idx;
        if (idx == null || u.cursor.left == null || u.cursor.left < 0) {
          tip.classList.add("hidden");
          return;
        }
        const { series, formatValue } = current();
        const time = document.createElement("div");
        time.className = "mb-1 text-zinc-500";
        time.textContent = new Date(
          (u.data[0][idx] ?? 0) * 1000,
        ).toLocaleTimeString();
        const rows = series.map((s, i) => {
          const v = u.data[i + 1]?.[idx];
          const row = document.createElement("div");
          row.className = "flex items-center gap-2 py-0.5";
          const key = document.createElement("span");
          key.className = "h-0.5 w-3 shrink-0 rounded-full";
          key.style.background = s.color;
          const value = document.createElement("span");
          value.className = "w-20 font-semibold tabular-nums";
          value.textContent = v == null ? "—" : formatValue(v);
          const label = document.createElement("span");
          label.className = "text-zinc-500";
          label.textContent = s.label;
          row.append(key, value, label);
          return row;
        });
        tip.replaceChildren(time, ...rows);
        tip.classList.remove("hidden");
        // Flip to the left of the cursor near the right edge.
        const left = u.cursor.left;
        const flip = left > u.over.clientWidth - 200;
        tip.style.left = `${flip ? left - tip.offsetWidth - 12 : left + 12}px`;
        tip.style.top = "8px";
      },
    },
  };
}
