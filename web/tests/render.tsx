import { QueryClient } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import { createMemoryRouter } from "react-router";
import { createApi } from "@/api/client";
import { Providers } from "@/app/providers";
import { routes } from "@/app/router";
import { i18next } from "@/i18n/i18n";
import type { Language } from "@/i18n/language";

export async function renderApp(path: string, language: Language = "en") {
  await i18next.changeLanguage(language);
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity, refetchOnWindowFocus: false } },
  });
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  window.history.replaceState(null, "", path);
  const api = createApi(queryClient, (to) => {
    void router.navigate(to);
  });
  const view = render(<Providers queryClient={queryClient} api={api} router={router} />);
  return { ...view, queryClient, router, api };
}

export function location(router: ReturnType<typeof createMemoryRouter>): string {
  return router.state.location.pathname + router.state.location.search;
}

// Matches the element that holds the whole sentence, whose dates sit in nowrap spans of their own.
export function sentence(text: string | RegExp) {
  return (_content: string, element: Element | null): boolean =>
    element !== null &&
    element.querySelector(":scope > span.whitespace-nowrap") !== null &&
    (typeof text === "string" ? element.textContent === text : text.test(element.textContent));
}
