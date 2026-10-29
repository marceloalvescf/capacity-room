import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type InfiniteData,
  type UseInfiniteQueryResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useApi } from "@/api/context";
import { ApiFailure } from "@/api/client";
import type { components } from "@/api/schema.gen";
import type { Principal } from "@/session/session";

type Schemas = components["schemas"];
export type Report = Schemas["Report"];
export type ReportPage = Schemas["ReportPage"];
export type ReportSummary = Schemas["ReportSummary"];
export type ReportSelection = Schemas["ReportSelection"];
export type ReportPDFState = Schemas["ReportPDFState"];
export type ProfileSummary = Schemas["ProfileSummary"];

export const reportsKey = ["reports"] as const;
export const profilesKey = ["profiles"] as const;

export function isActive(report: { status: Schemas["ReportStatus"]; pdfs: ReportPDFState[] }): boolean {
  return (
    report.status === "queued" || report.status === "running" || report.pdfs.some((pdf) => pdf.status === "pending")
  );
}

export function canDelete(principal: Principal, report: { owner_id: string }): boolean {
  return principal.role === "administrator" || (principal.role === "operator" && report.owner_id === principal.id);
}

function ended(error: unknown): boolean {
  return error instanceof ApiFailure && (error.status === 404 || error.status === 410);
}

function nextCheck(checks: number): number {
  if (checks < 20) {
    return 1500;
  }
  return checks < 40 ? 3000 : 6000;
}

export function useReports(): UseInfiniteQueryResult<InfiniteData<ReportPage>> {
  const api = useApi();
  return useInfiniteQuery({
    queryKey: reportsKey,
    initialPageParam: null as string | null,
    queryFn: async ({ pageParam }) => {
      const { data } = await api.GET("/reports", {
        params: { query: { limit: 50, ...(pageParam === null ? {} : { cursor: pageParam }) } },
      });
      if (data === undefined) {
        throw new Error("the reports answer is empty");
      }
      return data;
    },
    getNextPageParam: (last) => last.next_cursor,
    refetchInterval: (query) =>
      query.state.data?.pages.some((page) => page.items.some(isActive)) === true ? 5000 : false,
  });
}

// The update count at which each cached report query was first seen active.
// It lives with the query, so a screen that leaves and returns keeps its count.
const activeSince = new WeakMap<object, number>();

// A screen that is deleting the report turns the checking off, so that none
// reaches the report after it is gone.
export function useReport(id: string, enabled = true): UseQueryResult<Report> {
  const api = useApi();
  return useQuery({
    queryKey: [...reportsKey, id],
    enabled,
    queryFn: async () => {
      const { data } = await api.GET("/reports/{id}", { params: { path: { id } } });
      if (data === undefined) {
        throw new Error("the report answer is empty");
      }
      return data;
    },
    refetchInterval: (query) => {
      if (ended(query.state.error) || query.state.data === undefined || !isActive(query.state.data)) {
        activeSince.delete(query);
        return false;
      }
      // The checks count from the answer that showed the report active again, so
      // a PDF requested after a long run is followed as closely as a new report.
      const base = activeSince.get(query) ?? query.state.dataUpdateCount - 1;
      activeSince.set(query, base);
      return nextCheck(query.state.dataUpdateCount - base);
    },
  });
}

export function useProfiles(): UseQueryResult<ProfileSummary[]> {
  const api = useApi();
  return useQuery({
    queryKey: profilesKey,
    queryFn: async () => {
      const { data } = await api.GET("/profiles");
      if (data === undefined) {
        throw new Error("the profiles answer is empty");
      }
      return data.profiles;
    },
  });
}

// The namespaces the source of a profile shows. A failure of the source is an answer with a code other than
// PROFILE_OK, so a thrown error is the call itself failing.
export function useProfileNamespaces(id: string): UseQueryResult<Schemas["ProfileNamespaces"]> {
  const api = useApi();
  return useQuery({
    queryKey: [...profilesKey, id, "namespaces"],
    staleTime: 0,
    refetchOnWindowFocus: false,
    queryFn: async () => {
      const { data } = await api.GET("/profiles/{id}/namespaces", { params: { path: { id } } });
      if (data === undefined) {
        throw new Error("the namespaces answer is empty");
      }
      return data;
    },
  });
}

export function useRequestReport() {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ body, key }: { body: Schemas["ReportRequest"]; key: string }) => {
      const { data } = await api.POST("/reports", { params: { header: { "Idempotency-Key": key } }, body });
      if (data === undefined) {
        throw new Error("the request answer is empty");
      }
      return data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: reportsKey }),
  });
}

export function useRequestPDF(id: string) {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (language: Schemas["Language"]) => {
      const { data } = await api.POST("/reports/{id}/pdfs", { params: { path: { id } }, body: { language } });
      if (data === undefined) {
        throw new Error("the PDF answer is empty");
      }
      return data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: reportsKey }),
  });
}

// The list reloads whatever the answer was, so that a row already gone does not
// stay to be tried again; the deleted reports' own queries go, so that none
// checks a report that is no longer there.
function settle(queryClient: QueryClient, gone: string[]): Promise<void> {
  for (const id of gone) {
    queryClient.removeQueries({ queryKey: [...reportsKey, id], exact: true });
  }
  return queryClient.invalidateQueries({ queryKey: reportsKey, exact: true });
}

export function useDeleteReport() {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await api.DELETE("/reports/{id}", { params: { path: { id } } });
      return id;
    },
    onSettled: (_id, error, requested) => settle(queryClient, error === null ? [requested] : []),
  });
}

// The contract takes at most 200 ids in one request.
const deleteChunk = 200;

export function useDeleteReports() {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (ids: string[]) => {
      const results: Schemas["ReportDeleteResult"][] = [];
      for (let start = 0; start < ids.length; start += deleteChunk) {
        try {
          const { data } = await api.POST("/reports/delete", { body: { ids: ids.slice(start, start + deleteChunk) } });
          if (data === undefined) {
            throw new Error("the delete answer is empty");
          }
          results.push(...data.results);
        } catch (failure) {
          // The chunks already answered stand; the rest were not deleted.
          return { results, failure };
        }
      }
      return { results, failure: null as unknown };
    },
    onSettled: (outcome) =>
      settle(
        queryClient,
        (outcome?.results ?? []).filter((result) => result.outcome !== "forbidden").map((result) => result.id),
      ),
  });
}
