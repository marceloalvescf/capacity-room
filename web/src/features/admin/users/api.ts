import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { useApi } from "@/api/context";
import type { components } from "@/api/schema.gen";

type Schemas = components["schemas"];
export type User = Schemas["User"];
export type NewUser = Schemas["NewUser"];
export type UserChange = Schemas["UserChange"];

export const adminUsersKey = ["admin", "users"] as const;

export function useAdminUsers(): UseQueryResult<User[]> {
  const api = useApi();
  return useQuery({
    queryKey: adminUsersKey,
    queryFn: async () => {
      const { data } = await api.GET("/admin/users");
      if (data === undefined) {
        throw new Error("the users answer is empty");
      }
      return data.users;
    },
  });
}

export function useCreateUser() {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: NewUser) => {
      const { data } = await api.POST("/admin/users", { body });
      if (data === undefined) {
        throw new Error("the user answer is empty");
      }
      return data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: adminUsersKey }),
  });
}

export function useChangeUser(id: string) {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: UserChange) => {
      const { data } = await api.PATCH("/admin/users/{id}", { params: { path: { id } }, body });
      if (data === undefined) {
        throw new Error("the user answer is empty");
      }
      return data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: adminUsersKey }),
  });
}

export function useDeactivateUser(id: string) {
  const api = useApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await api.DELETE("/admin/users/{id}", { params: { path: { id } } });
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: adminUsersKey }),
  });
}
