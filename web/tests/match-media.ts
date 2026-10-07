import { vi } from "vitest";

type Listener = () => void;

// Installs a matchMedia that answers the dark colour-scheme query and lets a
// test flip the answer and notify the listeners.
export function mockColourScheme(initiallyDark: boolean): { setDark: (dark: boolean) => void } {
  let dark = initiallyDark;
  const listeners = new Set<Listener>();
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({
      get matches() {
        return query === "(prefers-color-scheme: dark)" && dark;
      },
      media: query,
      addEventListener: (_type: string, listener: Listener) => listeners.add(listener),
      removeEventListener: (_type: string, listener: Listener) => listeners.delete(listener),
    })),
  );
  return {
    setDark(value: boolean) {
      dark = value;
      listeners.forEach((listener) => {
        listener();
      });
    },
  };
}
