const memory = new Map<string, string>();

// A browser may refuse storage; the choice then lives in memory for the page.
export function readStored(key: string): string | null {
  let value: string | null = null;
  try {
    value = window.localStorage.getItem(key);
  } catch {
    value = null;
  }
  return value ?? memory.get(key) ?? null;
}

export function writeStored(key: string, value: string): void {
  memory.set(key, value);
  try {
    window.localStorage.setItem(key, value);
  } catch {
    return;
  }
}

export function removeStored(key: string): void {
  memory.delete(key);
  try {
    window.localStorage.removeItem(key);
  } catch {
    return;
  }
}
