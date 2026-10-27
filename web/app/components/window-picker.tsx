export interface WindowOption {
  label: string;
  ms: number;
}

export const WINDOWS: WindowOption[] = [
  { label: "1 min", ms: 60_000 },
  { label: "5 min", ms: 5 * 60_000 },
  { label: "15 min", ms: 15 * 60_000 },
  { label: "1 hour", ms: 60 * 60_000 },
];

// One time-window control per page, so every chart on it shows the same
// slice of time.
export function WindowPicker({
  value,
  onChange,
  options = WINDOWS,
}: {
  value: number;
  onChange: (ms: number) => void;
  options?: WindowOption[];
}) {
  return (
    <fieldset className="flex w-fit gap-0.5 rounded-xl bg-zinc-200/70 p-1 text-xs dark:bg-zinc-800/80">
      <legend className="sr-only">Time window</legend>
      {options.map((w) => (
        <button
          key={w.ms}
          type="button"
          aria-pressed={w.ms === value}
          onClick={() => onChange(w.ms)}
          className={`rounded-lg px-3 py-1.5 font-medium transition-[background-color,color,transform] active:scale-[0.97] ${
            w.ms === value
              ? "bg-white text-zinc-900 shadow-sm shadow-zinc-950/10 dark:bg-zinc-950 dark:text-zinc-50"
              : "text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
          }`}
        >
          {w.label}
        </button>
      ))}
    </fieldset>
  );
}
