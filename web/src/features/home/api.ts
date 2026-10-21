import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useApi } from "@/api/context";
import type { components } from "@/api/schema.gen";
import { isActive } from "@/features/reports/api";

export type Home = components["schemas"]["Home"];

export const homeKey = ["home"] as const;

// The answer is fetched again on every visit, and every 5 seconds while any cluster has a request in flight.
export function useHome(): UseQueryResult<Home> {
  const api = useApi();
  return useQuery({
    queryKey: homeKey,
    queryFn: async () => {
      const { data } = await api.GET("/home");
      if (data === undefined) {
        throw new Error("the home answer is empty");
      }
      return data;
    },
    staleTime: 0,
    refetchOnMount: "always",
    refetchInterval: (query) =>
      query.state.data?.clusters.some((c) => c.newest !== null && isActive(c.newest)) === true ? 5000 : false,
  });
}
