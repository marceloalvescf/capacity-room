import { Navigate, type RouteObject } from "react-router";
import { AccountPage } from "@/features/account/account-page";
import { LoginPage } from "@/features/auth/login-page";
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
          { index: true, element: <Navigate to="/account" replace /> },
          { path: "account", element: <AccountPage /> },
          { path: "*", element: <NotFound /> },
        ],
      },
    ],
  },
];
