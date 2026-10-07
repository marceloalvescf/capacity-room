import { QueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { createBrowserRouter } from "react-router";
import { createApi } from "@/api/client";
import { Providers } from "./providers";
import { routes } from "./router";

function createRuntime() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity, refetchOnWindowFocus: false } },
  });
  const router = createBrowserRouter(routes);
  const api = createApi(queryClient, (to) => {
    void router.navigate(to);
  });
  return { queryClient, router, api };
}

export function App() {
  const [runtime] = useState(createRuntime);
  return <Providers {...runtime} />;
}
