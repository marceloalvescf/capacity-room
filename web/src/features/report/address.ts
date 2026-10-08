import { useCallback, useEffect, useRef, useState } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router";

const canonical = (search: string): string => new URLSearchParams(search).toString();

export interface Address {
  // The parameters of the address as the last render saw them.
  params: URLSearchParams;
  // Changes the address from the latest address written, not from the last render, so that two writes in a row keep
  // both changes however late the router lands the first.
  write: (change: (next: URLSearchParams) => void) => void;
  // Counts the changes of the address that did not come from write: back, forward, a link.
  outside: number;
}

export function useAddress(): Address {
  const [params] = useSearchParams();
  const { search } = useLocation();
  const navigate = useNavigate();
  const live = useRef(canonical(search));
  const pending = useRef<string[]>([]);
  const [outside, setOutside] = useState(0);

  useEffect(() => {
    const landed = canonical(search);
    const at = pending.current.indexOf(landed);
    if (at >= 0) {
      pending.current = pending.current.slice(at + 1);
      return;
    }
    pending.current = [];
    if (landed !== live.current) {
      live.current = landed;
      setOutside((count) => count + 1);
    }
  }, [search]);

  const write = useCallback(
    (change: (next: URLSearchParams) => void) => {
      const next = new URLSearchParams(live.current);
      change(next);
      const text = next.toString();
      if (text === live.current) {
        return;
      }
      live.current = text;
      pending.current = [...pending.current, text];
      void navigate({ search: text }, { replace: true });
    },
    [navigate],
  );

  return { params, write, outside };
}
