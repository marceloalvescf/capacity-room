import type { QueryClient } from "@tanstack/react-query";
import createClient from "openapi-fetch";
import { currentCsrf, sessionKey } from "@/session/session";
import { errorCodeValues, type components, type paths } from "./schema.gen";

export type ErrorCode = components["schemas"]["ErrorCode"];

// The client adds the CSRF header to every state-changing call, so a caller
// never names it.
type CsrfHeader = "X-CSRF-Token";
type WithoutCsrf<Operation> = Operation extends { parameters: infer Parameters }
  ? Omit<Operation, "parameters"> & {
      parameters: Omit<Parameters, "header"> &
        (Parameters extends { header: infer Header }
          ? keyof Omit<Header, CsrfHeader> extends never
            ? { header?: never }
            : { header: Omit<Header, CsrfHeader> }
          : unknown);
    }
  : Operation;
type ApiPaths = { [Path in keyof paths]: { [Method in keyof paths[Path]]: WithoutCsrf<paths[Path][Method]> } };

export type Api = ReturnType<typeof createClient<ApiPaths>>;

export class ApiFailure extends Error {
  readonly status: number;
  readonly code: ErrorCode | null;
  readonly retryAfter: number | null;
  readonly fields: Readonly<Record<string, string>>;

  constructor(
    status: number,
    code: ErrorCode | null,
    retryAfter: number | null,
    fields: Readonly<Record<string, string>> = {},
  ) {
    super(`request failed with status ${String(status)}`);
    this.status = status;
    this.code = code;
    this.retryAfter = retryAfter;
    this.fields = fields;
  }
}

const apiPath = "/api/v1";
const readOnly = new Set(["GET", "HEAD", "OPTIONS"]);

function isErrorCode(value: unknown): value is ErrorCode {
  return errorCodeValues.some((code) => code === value);
}

interface ErrorDetail {
  code: ErrorCode | null;
  fields: Record<string, string>;
}

async function detailOf(response: Response): Promise<ErrorDetail> {
  try {
    const body = (await response.clone().json()) as { error?: { code?: unknown; fields?: unknown } } | null;
    const code = body?.error?.code;
    const raw = body?.error?.fields;
    const fields: Record<string, string> = {};
    if (typeof raw === "object" && raw !== null) {
      for (const [name, value] of Object.entries(raw)) {
        if (typeof value === "string") {
          fields[name] = value;
        }
      }
    }
    return { code: isErrorCode(code) ? code : null, fields };
  } catch {
    return { code: null, fields: {} };
  }
}

async function codeOf(response: Response): Promise<ErrorCode | null> {
  return (await detailOf(response)).code;
}

function retryAfterOf(response: Response): number | null {
  const seconds = Number.parseInt(response.headers.get("Retry-After") ?? "", 10);
  return Number.isFinite(seconds) ? seconds : null;
}

// A 401 on sign-out means the reader is already signed out; the caller ends it.
function isLogout(request: Request): boolean {
  return request.method === "POST" && new URL(request.url).pathname === `${apiPath}/auth/logout`;
}

export function createApi(queryClient: QueryClient, navigate: (to: string) => void): Api {
  async function dispatch(request: Request): Promise<Response> {
    if (!readOnly.has(request.method)) {
      const token = currentCsrf(queryClient);
      if (token !== null) {
        request.headers.set("X-CSRF-Token", token);
      }
    }
    return globalThis.fetch(request);
  }

  async function refreshSession(): Promise<void> {
    const { data } = await client.GET("/auth/session");
    if (data !== undefined) {
      queryClient.setQueryData(sessionKey, data);
    }
  }

  async function send(request: Request): Promise<Response> {
    const again = request.clone();
    const first = await dispatch(request);
    if (first.status !== 403 || readOnly.has(request.method) || (await codeOf(first)) !== "CSRF_REJECTED") {
      return first;
    }
    await refreshSession();
    return dispatch(again);
  }

  async function guarded(request: Request): Promise<Response> {
    const response = await send(request);
    if (response.ok) {
      return response;
    }
    const { code, fields } = await detailOf(response);
    if (response.status === 401 && code !== "INVALID_CREDENTIALS" && !isLogout(request)) {
      const here = window.location.pathname;
      const next = encodeURIComponent(here + window.location.search);
      queryClient.clear();
      if (here !== "/login") {
        navigate(`/login?next=${next}&ended=1`);
      }
    }
    throw new ApiFailure(response.status, code, retryAfterOf(response), fields);
  }

  const client = createClient<ApiPaths>({
    baseUrl: `${window.location.origin}${apiPath}`,
    fetch: guarded,
  });
  return client;
}
