import { Navigate, type RouteObject } from "react-router";
import { AccountPage } from "@/features/account/account-page";
import { LoginPage } from "@/features/auth/login-page";
import { AutoscalingTab } from "@/features/report/autoscaling-tab";
import { NamespacesTab } from "@/features/report/namespaces-tab";
import { NodesTab } from "@/features/report/nodes-tab";
import { OverviewTab } from "@/features/report/overview-tab";
import { ReportPage } from "@/features/report/report-page";
import { FindingsTab } from "@/features/report/findings-tab";
import { WorkloadsTab } from "@/features/report/workloads-tab";
import { GeneratePage } from "@/features/reports/generate-page";
import { ProgressPage } from "@/features/reports/progress-page";
import { ReportsPage } from "@/features/reports/reports-page";
import { RequireOperator } from "@/session/require-operator";
import { RequireSession } from "@/session/require-session";
import { NotFound } from "./not-found";
import { Shell } from "./shell";

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
              { path: "overview", element: <OverviewTab /> },
              { path: "findings", element: <FindingsTab /> },
              { path: "namespaces", element: <NamespacesTab /> },
              { path: "workloads", element: <WorkloadsTab /> },
              { path: "autoscaling", element: <AutoscalingTab /> },
              { path: "nodes", element: <NodesTab /> },
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
