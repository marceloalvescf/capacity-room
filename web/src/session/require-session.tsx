import { useTranslation } from "react-i18next";
import { Navigate, Outlet, useLocation } from "react-router";
import { errorText, toError } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { isSignedIn, useSession } from "./session";

export function RequireSession() {
  const { t } = useTranslation();
  const { pathname, search } = useLocation();
  const { data, error } = useSession();

  if (error !== null) {
    return <Alert tone="error">{errorText(t, toError(error))}</Alert>;
  }
  if (data === undefined) {
    return null;
  }
  if (isSignedIn(data)) {
    return <Outlet />;
  }
  const here = pathname + search;
  return <Navigate to={here === "/" ? "/login" : `/login?next=${encodeURIComponent(here)}`} replace />;
}
