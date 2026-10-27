import { useCallback, useEffect, useId, useRef, useState } from "react";
import { Link } from "react-router";
import { GLOSSARY, type TermId } from "~/features/learn/glossary";
import { InfoIcon } from "./icons";

const WIDTH = 288; // px, w-72
const MARGIN = 8;
// Grace period for moving the pointer from the icon onto the panel (and
// back) without it closing.
const CLOSE_DELAY_MS = 200;

// A small ⓘ that explains a term in plain language. It opens on hover and
// stays open while the pointer is on the icon or the panel, so the text can
// be read and its link clicked. Keyboard focus opens it too, and on touch
// screens (no hover) a tap toggles it. The panel is position: fixed so
// tables that scroll sideways can't clip it.
export function InfoTip({ id, label }: { id: TermId; label?: string }) {
  const entry = GLOSSARY[id];
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState({ top: 0, left: 0 });
  const button = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const closeTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const panelId = useId();

  const cancelClose = () => clearTimeout(closeTimer.current);
  const show = () => {
    cancelClose();
    const r = button.current?.getBoundingClientRect();
    if (!r) return;
    const left = Math.min(
      Math.max(r.left + r.width / 2 - WIDTH / 2, MARGIN),
      window.innerWidth - WIDTH - MARGIN,
    );
    setPos({ top: r.bottom + MARGIN, left });
    setOpen(true);
  };
  const hide = useCallback(() => {
    clearTimeout(closeTimer.current);
    setOpen(false);
  }, []);
  const hideSoon = () => {
    cancelClose();
    closeTimer.current = setTimeout(hide, CLOSE_DELAY_MS);
  };

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && hide();
    // Touch screens have no "pointer left", so a tap elsewhere closes it.
    const onPointer = (e: PointerEvent) => {
      const t = e.target as Node;
      if (!button.current?.contains(t) && !panel.current?.contains(t)) hide();
    };
    window.addEventListener("keydown", onKey);
    window.addEventListener("pointerdown", onPointer);
    window.addEventListener("scroll", hide, true);
    window.addEventListener("resize", hide);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("pointerdown", onPointer);
      window.removeEventListener("scroll", hide, true);
      window.removeEventListener("resize", hide);
    };
  }, [open, hide]);

  useEffect(() => () => clearTimeout(closeTimer.current), []);

  const mouse = (e: React.PointerEvent) => e.pointerType === "mouse";

  return (
    <>
      <button
        ref={button}
        type="button"
        aria-label={`What is ${label ?? entry.term}?`}
        aria-expanded={open}
        aria-controls={panelId}
        onPointerEnter={(e) => mouse(e) && show()}
        onPointerLeave={(e) => mouse(e) && hideSoon()}
        // Hover handles the mouse; a tap on a touch screen toggles.
        onPointerUp={(e) => {
          if (mouse(e)) return;
          if (open) hide();
          else show();
        }}
        onFocus={show}
        onBlur={(e) => {
          if (!panel.current?.contains(e.relatedTarget)) hideSoon();
        }}
        className="inline-flex size-5 shrink-0 cursor-help items-center justify-center rounded-full align-middle text-zinc-400 transition-colors hover:text-zinc-700 dark:text-zinc-500 dark:hover:text-zinc-200"
      >
        <InfoIcon className="size-4" />
      </button>
      {open && (
        <div
          ref={panel}
          id={panelId}
          role="tooltip"
          onPointerEnter={cancelClose}
          onPointerLeave={(e) => mouse(e) && hideSoon()}
          onBlur={(e) => {
            if (
              !panel.current?.contains(e.relatedTarget) &&
              e.relatedTarget !== button.current
            ) {
              hideSoon();
            }
          }}
          style={{ top: pos.top, left: pos.left, width: WIDTH }}
          className="fixed z-50 rounded-xl bg-zinc-900 p-3.5 text-left text-[13px] leading-relaxed font-normal text-zinc-100 shadow-lg shadow-zinc-950/20 dark:bg-zinc-800 dark:ring-1 dark:ring-white/10"
        >
          <p className="mb-1 font-semibold text-white">{entry.term}</p>
          <p className="text-zinc-300">{entry.short}</p>
          <Link
            to={`/learn#${id}`}
            onClick={hide}
            className="mt-2 inline-block text-xs font-medium text-sky-300 hover:underline"
          >
            More in Learn
          </Link>
        </div>
      )}
    </>
  );
}

// A term followed by its info tip, e.g. <Term id="jitter">Jitter</Term>.
export function Term({
  id,
  children,
}: {
  id: TermId;
  children: React.ReactNode;
}) {
  return (
    <span className="inline-flex items-center gap-1">
      {children}
      <InfoTip
        id={id}
        label={typeof children === "string" ? children : undefined}
      />
    </span>
  );
}
