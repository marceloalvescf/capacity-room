import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useApi } from "@/api/context";
import type { components } from "@/api/schema.gen";
import { profilesKey } from "@/features/reports/api";

type Schemas = components["schemas"];
export type Profile = Schemas["Profile"];
export type ProfileTestResult = Schemas["ProfileTestResult"];

export const adminProfilesKey = ["admin", "profiles"] as const;

export function useAdminProfiles(): UseQueryResult<Profile[]> {
  const api = useApi();
  return useQuery({
    queryKey: adminProfilesKey,
    queryFn: async () => {
      const { data } = await api.GET("/admin/profiles");
      if (data === undefined) {
        throw new Error("the profiles answer is empty");
      }
      return data.profiles;
    },
  });
}

export function useAdminProfile(id: string): UseQueryResult<Profile> {
  const api = useApi();
  return useQuery({
    queryKey: [...adminProfilesKey, id],
    queryFn: async () => {
      const { data } = await api.GET("/admin/profiles/{id}", { params: { path: { id } } });
      if (data === undefined) {
        throw new Error("the profile answer is empty");
      }
      return data;
    },
  });
}

export function useDeleteProfile(id: string) {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await api.DELETE("/admin/profiles/{id}", { params: { path: { id } } });
    },
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: adminProfilesKey }),
        queryClient.invalidateQueries({ queryKey: profilesKey }),
      ]),
  });
}

// Runs the preflight against the source of the saved revision.
export function useTestProfile(id: string) {
  const api = useApi();
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.POST("/admin/profiles/{id}/test", { params: { path: { id } } });
      if (data === undefined) {
        throw new Error("the test answer is empty");
      }
      return data;
    },
  });
}
