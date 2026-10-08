import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useApi } from "@/api/context";
import type { components } from "@/api/schema.gen";
import type { Language } from "@/i18n/language";
import { reportsKey } from "@/features/reports/api";

type Schemas = components["schemas"];
export type ReportView = Schemas["ReportView"];
export type ViewFinding = Schemas["ViewFinding"];
export type ViewValue = Schemas["ViewValue"];
export type ViewStatistic = Schemas["ViewStatistic"];
export type ViewNamespace = Schemas["ViewNamespace"];
export type ViewWorkload = Schemas["ViewWorkload"];
export type ViewHPA = Schemas["ViewHPA"];
export type ViewNode = Schemas["ViewNode"];
export type ViewNodepool = Schemas["ViewNodepool"];
export type ViewCapability = Schemas["ViewCapability"];
export type ViewMessage = Schemas["ViewMessage"];

// The view of a report in one language. It is read once: the snapshot behind it never changes.
export function useReportView(id: string, language: Language): UseQueryResult<ReportView> {
  const api = useApi();
  return useQuery({
    queryKey: [...reportsKey, id, "view", language],
    queryFn: async () => {
      const { data } = await api.GET("/reports/{id}/view", { params: { path: { id }, query: { lang: language } } });
      if (data === undefined) {
        throw new Error("the view answer is empty");
      }
      return data;
    },
  });
}
