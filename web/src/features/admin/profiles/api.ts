import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useApi } from "@/api/context";
import type { components } from "@/api/schema.gen";
import { profilesKey } from "@/features/reports/api";

type Schemas = components["schemas"];
// The contract requires every field of a profile as the administration reads it; the generated type loses that
// across `allOf`, so it is restored here once.
export type ProfileDocument = Required<Schemas["ProfileDocument"]> & { thresholds: Required<Schemas["ProfileThresholds"]> };
export type Profile = Schemas["Profile"] & ProfileDocument;
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
      return data.profiles as Profile[];
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
      return data as Profile;
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

export function useCreateProfile() {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: Schemas["ProfileCreate"]) => {
      const { data } = await api.POST("/admin/profiles", { body });
      if (data === undefined) {
        throw new Error("the profile answer is empty");
      }
      return data as Profile;
    },
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: adminProfilesKey }),
        queryClient.invalidateQueries({ queryKey: profilesKey }),
      ]),
  });
}

export function useReviseProfile(id: string) {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: Schemas["ProfileChange"]) => {
      const { data } = await api.PATCH("/admin/profiles/{id}", { params: { path: { id } }, body });
      if (data === undefined) {
        throw new Error("the profile answer is empty");
      }
      return data as Profile;
    },
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: adminProfilesKey }),
        queryClient.invalidateQueries({ queryKey: profilesKey }),
      ]),
  });
}
