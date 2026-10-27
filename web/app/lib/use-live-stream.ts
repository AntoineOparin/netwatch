import { useEffect, useRef, useState } from "react";

const RECONNECT_MS = 2000;

// Follows a server-streaming RPC that sends history first and then live
// points. Each message is folded into state with `reduce`; on a fresh
// connection `reduce` gets `undefined` so the history replaces stale state.
// If the stream drops (hub restart), it reconnects every 2 seconds. Changing
// `key` (e.g. the time window) opens a fresh stream.
export function useLiveStream<S, P>(
  open: (signal: AbortSignal) => AsyncIterable<{ points: P[] }>,
  reduce: (state: S | undefined, points: P[]) => S,
  initial: S,
  key: string | number,
) {
  const [data, setData] = useState<S>(initial);
  const [connected, setConnected] = useState(false);
  // Read through refs so callers can pass inline functions.
  const fns = useRef({ open, reduce });
  fns.current = { open, reduce };

  // biome-ignore lint/correctness/useExhaustiveDependencies: key is the reconnect trigger; open and reduce are read through a ref.
  useEffect(() => {
    const abort = new AbortController();

    (async () => {
      while (!abort.signal.aborted) {
        try {
          let state: S | undefined;
          for await (const res of fns.current.open(abort.signal)) {
            state = fns.current.reduce(state, res.points);
            setData(state);
            setConnected(true);
          }
        } catch {
          // Aborted on unmount, or the hub went away: fall through to retry.
        }
        if (abort.signal.aborted) return;
        setConnected(false);
        await new Promise((r) => setTimeout(r, RECONNECT_MS));
      }
    })();

    return () => abort.abort();
  }, [key]);

  return { data, connected };
}
