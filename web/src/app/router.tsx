import { useTranslation } from "react-i18next";
import { Navigate, type RouteObject } from "react-router";
import { AccountPage } from "@/features/account/account-page";
import { LoginPage } from "@/features/auth/login-page";
import { ReportPage, useView } from "@/features/report/report-page";
import { GeneratePage } from "@/features/reports/generate-page";
import { ProgressPage } from "@/features/reports/progress-page";
import { ReportsPage } from "@/features/reports/reports-page";
import { RequireOperator } from "@/session/require-operator";
import { RequireSession } from "@/session/require-session";
import { NotFound } from "./not-found";
import { Shell } from "./shell";

function OverviewPlaceholder() {
  const { t } = useTranslation();
  useView();
  return <h2 className="text-base font-semibold">{t("report.tab.overview")}</h2>;
}

export const routes: RouteObject[] = [
  { path: "/login", element: <LoginPage /> },
  {
    element: <RequireSession />,
    children: [
      {
        element: <Shell />,
        children: [
          { index: true, element: <Navigate to="/reports" replace /> },
          { path: "reports", element: <ReportsPage /> },
          {
            path: "generate",
            element: <RequireOperator />,
            children: [{ index: true, element: <GeneratePage /> }],
          },
          { path: "jobs/:id", element: <ProgressPage /> },
          {
            path: "reports/:id",
            element: <ReportPage />,
            children: [
              { index: true, element: <Navigate to="overview" replace /> },
              { path: "overview", element: <OverviewPlaceholder /> },
              { path: "*", element: <NotFound /> },
            ],
          },
          { path: "account", element: <AccountPage /> },
          { path: "*", element: <NotFound /> },
        ],
      },
    ],
  },
];
