import type { HomeCluster } from "./clusters";

const keyOf = (userId: string) => `capacityroom-home-seen:${userId}`;

// The map of profile id to the report id last shown, or null when storage is refused.
// Storage is read directly: a refusal must read as seen, which the shared store's memory fallback would hide.
function read(userId: string): Record<string, string> | null {
  try {
    const raw = window.localStorage.getItem(keyOf(userId));
    if (raw === null) {
      return {};
    }
    const parsed: unknown = JSON.parse(raw);
    return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)
      ? (parsed as Record<string, string>)
      : {};
  } catch {
    return null;
  }
}

function write(userId: string, map: Record<string, string>): void {
  try {
    window.localStorage.setItem(keyOf(userId), JSON.stringify(map));
  } catch {
    return;
  }
}

// Whether the cluster's latest report is not the one last shown. A profile with no entry is recorded and reads as seen.
export function unseen(userId: string, c: HomeCluster): boolean {
  const map = read(userId);
  if (map === null || c.latest === null) {
    return false;
  }
  const last = map[c.profile_id];
  if (last === undefined) {
    write(userId, { ...map, [c.profile_id]: c.latest.id });
    return false;
  }
  return last !== c.latest.id;
}

export function markSeen(userId: string, profileId: string, reportId: string): void {
  const map = read(userId);
  if (map === null) {
    return;
  }
  write(userId, { ...map, [profileId]: reportId });
}
