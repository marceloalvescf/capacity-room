import { useQuery, type QueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useApi } from "@/api/context";
import type { components } from "@/api/schema.gen";

export type Session = components["schemas"]["Session"];
export type SignedInSession = components["schemas"]["SignedInSession"];
export type Principal = components["schemas"]["Principal"];

export const sessionKey = ["session"] as const;

export function isSignedIn(session: Session): session is SignedInSession {
  return "csrf_token" in session;
}

export function useSession(): UseQueryResult<Session> {
  const api = useApi();
  return useQuery({
    queryKey: sessionKey,
    queryFn: async () => {
      const { data } = await api.GET("/auth/session");
      if (data === undefined) {
        throw new Error("the session answer is empty");
      }
      return data;
    },
  });
}

export function usePrincipal(): Principal | null {
  const { data } = useSession();
  return data !== undefined && isSignedIn(data) ? data.principal : null;
}

// The token a state-changing request carries now: the sign-in one while
// anonymous, the session's once signed in.
export function currentCsrf(client: QueryClient): string | null {
  const session = client.getQueryData<Session>(sessionKey);
  if (session === undefined) {
    return null;
  }
  return isSignedIn(session) ? session.csrf_token : session.login_csrf;
}

// A sign-in redirect target stays on this origin: a path that starts with one
// slash, with no backslash or control character, that parses to this origin.
export function safeNext(raw: string | null): string {
  if (raw === null || !raw.startsWith("/") || raw.startsWith("//")) {
    return "/";
  }
  // eslint-disable-next-line no-control-regex
  if (/[\u0000-\u001f\u007f\\]/u.test(raw)) {
    return "/";
  }
  const base = "http://local.invalid";
  return new URL(raw, base).origin === base ? raw : "/";
}
