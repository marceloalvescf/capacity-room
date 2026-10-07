import { createContext, useCallback, useContext, useState, type ReactNode } from "react";

const AnnounceContext = createContext<(message: string) => void>(() => undefined);

export function useAnnounce(): (message: string) => void {
  return useContext(AnnounceContext);
}

// The one polite live region of the page.
export function LiveRegion({ children }: { children: ReactNode }) {
  const [message, setMessage] = useState("");
  const announce = useCallback((next: string) => {
    setMessage("");
    queueMicrotask(() => {
      setMessage(next);
    });
  }, []);

  return (
    <AnnounceContext value={announce}>
      {children}
      <div aria-live="polite" role="status" className="sr-only">
        {message}
      </div>
    </AnnounceContext>
  );
}
