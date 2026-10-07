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
export type ReportPDFState = Schemas["ReportPDFState"];
export type ProfileSummary = Schemas["ProfileSummary"];

export const reportsKey = ["reports"] as const;
const profilesKey = ["profiles"] as const;

export function isActive(report: { status: Schemas["ReportStatus"]; pdfs: ReportPDFState[] }): boolean {
  return report.status === "queued" || report.status === "running" || report.pdfs.some((pdf) => pdf.status === "pending");
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
    refetchInterval: (query) => (query.state.data?.pages.some((page) => page.items.some(isActive)) === true ? 5000 : false),
  });
}

export function useReport(id: string): UseQueryResult<Report> {
  const api = useApi();
  return useQuery({
    queryKey: [...reportsKey, id],
    queryFn: async () => {
      const { data } = await api.GET("/reports/{id}", { params: { path: { id } } });
      if (data === undefined) {
        throw new Error("the report answer is empty");
      }
      return data;
    },
    refetchInterval: (query) => {
      if (ended(query.state.error) || query.state.data === undefined || !isActive(query.state.data)) {
        return false;
      }
      return nextCheck(query.state.dataUpdateCount);
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

// The list reloads; the deleted reports' own queries go, so that none checks a
// report that is no longer there.
function forget(queryClient: QueryClient, ids: string[]): Promise<void> {
  for (const id of ids) {
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
    onSuccess: (id) => forget(queryClient, [id]),
  });
}

export function useDeleteReports() {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (ids: string[]) => {
      const { data } = await api.POST("/reports/delete", { body: { ids } });
      if (data === undefined) {
        throw new Error("the delete answer is empty");
      }
      return data.results;
    },
    onSuccess: (results) =>
      forget(
        queryClient,
        results.filter((result) => result.outcome !== "forbidden").map((result) => result.id),
      ),
  });
}
