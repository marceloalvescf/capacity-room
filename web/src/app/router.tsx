import { Navigate, type RouteObject } from "react-router";
import { AccountPage } from "@/features/account/account-page";
import { LoginPage } from "@/features/auth/login-page";
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
          { path: "account", element: <AccountPage /> },
          { path: "*", element: <NotFound /> },
        ],
      },
    ],
  },
];
