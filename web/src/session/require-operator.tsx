import { Outlet } from "react-router";
import { NotFound } from "@/app/not-found";
import { usePrincipal } from "./session";

// A reader has no screen that requests; the address answers as an unknown one.
export function RequireOperator() {
  const principal = usePrincipal();
  return principal !== null && principal.role !== "reader" ? <Outlet /> : <NotFound />;
}
