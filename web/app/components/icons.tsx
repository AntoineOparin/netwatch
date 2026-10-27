// A small hand-drawn icon set: 24px grid, 1.75 stroke, round caps, so every
// icon has the same weight.

type IconProps = { className?: string };

function Icon({
  className = "size-5",
  children,
}: IconProps & { children: React.ReactNode }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={className}
    >
      {children}
    </svg>
  );
}

// A pulse line: the overview's live signal.
export function PulseIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M3 12h4l2.5-6 5 12 2.5-6h4" />
    </Icon>
  );
}

// A jagged spike: lag incidents.
export function SpikeIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M3 17h4l3-10 4 10 2-5h5" />
      <path d="M3 21h18" />
    </Icon>
  );
}

// Nodes on a network: devices.
export function DevicesIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <rect x="9" y="3" width="6" height="5" rx="1.5" />
      <rect x="3" y="16" width="6" height="5" rx="1.5" />
      <rect x="15" y="16" width="6" height="5" rx="1.5" />
      <path d="M12 8v4M6 16v-2a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v2" />
    </Icon>
  );
}

// An open book: the Learn page.
export function BookIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M12 6.5C10.5 5 8 4.5 4 5v13c4-.5 6.5 0 8 1.5 1.5-1.5 4-2 8-1.5V5c-4-.5-6.5 0-8 1.5Z" />
      <path d="M12 6.5V19" />
    </Icon>
  );
}

// A window with a side panel: collapse or expand the sidebar.
export function SidebarIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <rect x="3" y="4" width="18" height="16" rx="3" />
      <path d="M9 4v16" />
    </Icon>
  );
}

export function InfoIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v5M12 8h.01" />
    </Icon>
  );
}

export function ArrowRightIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M5 12h14M13 6l6 6-6 6" />
    </Icon>
  );
}

// The NetWatch mark: a signal crossing three hops.
export function Logo({ className = "size-7" }: IconProps) {
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true" className={className}>
      <rect
        width="32"
        height="32"
        rx="9"
        className="fill-zinc-900 dark:fill-zinc-100"
      />
      <path
        d="M6 18h5l3-7 4 10 3-6h5"
        fill="none"
        strokeWidth="2.25"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="stroke-white dark:stroke-zinc-900"
      />
    </svg>
  );
}
