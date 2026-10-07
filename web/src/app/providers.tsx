import { QueryClientProvider, type QueryClient } from "@tanstack/react-query";
import { RouterProvider, type DataRouter } from "react-router";
import { ApiContext } from "@/api/context";
import type { Api } from "@/api/client";
import { SessionLanguage } from "@/session/session-language";
import { LiveRegion } from "./live-region";

export function Providers({ queryClient, api, router }: { queryClient: QueryClient; api: Api; router: DataRouter }) {
  return (
    <QueryClientProvider client={queryClient}>
      <ApiContext value={api}>
        <LiveRegion>
          <SessionLanguage />
          <RouterProvider router={router} />
        </LiveRegion>
      </ApiContext>
    </QueryClientProvider>
  );
}
