import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { NavLink, Outlet, useNavigate } from "react-router";
import { useApi } from "@/api/context";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { usePrincipal } from "@/session/session";
import { Brand } from "./brand";
import { LanguageMenu } from "./language-menu";
import { useAnnounce } from "./live-region";

export function Shell() {
  const { t } = useTranslation();
  const api = useApi();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const announce = useAnnounce();
  const principal = usePrincipal();

  if (principal === null) {
    return null;
  }

  async function signOut() {
    try {
      await api.POST("/auth/logout");
    } catch (thrown) {
      const error = toError(thrown);
      if (!(error instanceof ApiFailure && error.status === 401)) {
        announce(errorText(t, error));
        return;
      }
    }
    queryClient.clear();
    void navigate("/login");
  }

  return (
    <div className="min-h-screen bg-bg text-fg md:grid md:grid-cols-[16rem_1fr]">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-2 focus:top-2 focus:z-10 focus:rounded-md focus:bg-surface focus:px-3 focus:py-2"
      >
        {t("app.skip")}
      </a>
      <aside aria-label={t("app.name")} className="flex flex-col gap-6 bg-sidebar p-4 text-sidebar-fg">
        <Brand />
        <nav aria-label={t("nav.label")} className="flex-1">
          <ul>
            <li>
              <NavLink
                to="/account"
                className="block rounded-md px-3 py-2 text-sm motion-safe:transition-colors aria-[current=page]:bg-sidebar-active aria-[current=page]:text-sidebar-active-fg"
              >
                {t("nav.account")}
              </NavLink>
            </li>
          </ul>
        </nav>
        <div className="flex flex-col text-sm">
          <span className="font-mono">{principal.username}</span>
          <span>{t(`role.${principal.role}`)}</span>
        </div>
      </aside>
      <div className="flex min-w-0 flex-col">
        <header className="flex flex-wrap items-center justify-end gap-2 border-b border-border px-6 py-3">
          <LanguageMenu />
          <ThemeToggle />
          <Button
            onClick={() => {
              void signOut();
            }}
          >
            {t("header.signOut")}
          </Button>
        </header>
        <main id="main" tabIndex={-1} className="flex-1 p-6 outline-none">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
