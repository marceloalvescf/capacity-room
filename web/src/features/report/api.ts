import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useApi } from "@/api/context";
import type { components } from "@/api/schema.gen";
import type { Language } from "@/i18n/language";

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
export type ViewRecommendation = Schemas["ViewRecommendation"];

// The view of a report in one language. It is read once: the snapshot behind it never changes, so no change of
// the reports reloads it. The view of the previous language of the same report stays on screen while the new one
// loads.
export function useReportView(id: string, language: Language): UseQueryResult<ReportView> {
  const api = useApi();
  return useQuery({
    queryKey: ["report-view", id, language],
    placeholderData: (previous, previousQuery) => (previousQuery?.queryKey[1] === id ? previous : undefined),
    queryFn: async () => {
      const { data } = await api.GET("/reports/{id}/view", { params: { path: { id }, query: { lang: language } } });
      if (data === undefined) {
        throw new Error("the view answer is empty");
      }
      return data;
    },
  });
}
