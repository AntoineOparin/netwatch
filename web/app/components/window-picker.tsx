export const WINDOWS = [
  { label: "1 min", ms: 60_000 },
  { label: "5 min", ms: 5 * 60_000 },
  { label: "15 min", ms: 15 * 60_000 },
  { label: "1 hour", ms: 60 * 60_000 },
];

// One time-window control for the whole page, so every chart shows the
// same slice of time.
export function WindowPicker({
  value,
  onChange,
}: {
  value: number;
  onChange: (ms: number) => void;
}) {
  return (
    <fieldset className="flex w-fit gap-1 rounded-lg bg-gray-200/60 p-0.5 text-xs dark:bg-gray-800">
      <legend className="sr-only">Time window</legend>
      {WINDOWS.map((w) => (
        <button
          key={w.ms}
          type="button"
          aria-pressed={w.ms === value}
          onClick={() => onChange(w.ms)}
          className={`rounded-md px-2.5 py-1 font-medium ${
            w.ms === value
              ? "bg-white shadow-sm dark:bg-gray-950"
              : "text-gray-500 hover:text-gray-900 dark:hover:text-gray-100"
          }`}
        >
          {w.label}
        </button>
      ))}
    </fieldset>
  );
}
