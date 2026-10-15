import { SignOutIcon } from "@phosphor-icons/react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router";
import { useApi } from "@/api/context";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { ThemeToggle } from "@/components/theme-toggle";
import { IconButton } from "@/components/ui/icon-button";
import { usePrincipal } from "@/session/session";
import { Brand } from "./brand";
import { LanguageMenu } from "./language-menu";
import { useAnnounce } from "./live-region";

// The page in view: the active tint, semibold, and a neat-line frame on every side. Another page under the pointer
// takes a lighter tint.
const navLink =
  "block rounded-md px-3 py-2 text-sm not-aria-[current=page]:hover:bg-sidebar-hover not-aria-[current=page]:hover:text-sidebar-active-fg aria-[current=page]:bg-sidebar-active aria-[current=page]:font-semibold aria-[current=page]:ring-1 aria-[current=page]:ring-neatline aria-[current=page]:ring-inset aria-[current=page]:text-sidebar-active-fg";

const version = import.meta.env.VITE_CAPACITYROOM_VERSION;

export function Shell() {
  const { t } = useTranslation();
  const api = useApi();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const announce = useAnnounce();
  const principal = usePrincipal();
  // The list, a job in progress, and a report all sit under Reports.
  const inReports = /^\/(reports|jobs)(\/|$)/.test(useLocation().pathname);

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
      <aside aria-label={t("app.name")} className="flex flex-col gap-6 bg-sidebar p-4 text-sidebar-fg border-b border-neatline md:border-r md:border-b-0">
        <Brand />
        <nav aria-label={t("nav.label")} className="flex-1">
          <ul>
            <li>
              <Link to="/reports" aria-current={inReports ? "page" : undefined} className={navLink}>
                {t("nav.reports")}
              </Link>
            </li>
            {principal.role === "reader" ? null : (
              <li>
                <NavLink to="/generate" className={navLink}>
                  {t("nav.generate")}
                </NavLink>
              </li>
            )}
            {principal.role === "administrator" ? (
              <>
                <li>
                  <NavLink to="/profiles" className={navLink}>
                    {t("nav.profiles")}
                  </NavLink>
                </li>
                <li>
                  <NavLink to="/users" className={navLink}>
                    {t("nav.users")}
                  </NavLink>
                </li>
              </>
            ) : null}
            <li>
              <NavLink to="/account" className={navLink}>
                {t("nav.account")}
              </NavLink>
            </li>
          </ul>
        </nav>
        <div className="flex flex-col text-sm">
          <span className="font-mono">{principal.username}</span>
          <span>{t(`role.${principal.role}`)}</span>
          <span className="mt-3">
            {t("app.version")} <span className="font-mono">{version}</span>
          </span>
        </div>
      </aside>
      <div className="flex min-w-0 flex-col">
        <header className="flex flex-wrap items-center justify-end gap-2 border-b border-neatline px-6 py-3">
          <LanguageMenu />
          <ThemeToggle />
          <IconButton
            label={t("header.signOut")}
            onClick={() => {
              void signOut();
            }}
          >
            <SignOutIcon aria-hidden="true" className="size-5" />
          </IconButton>
        </header>
        <main id="main" tabIndex={-1} className="flex-1 p-6 outline-none">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
