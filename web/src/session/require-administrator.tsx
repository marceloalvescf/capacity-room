import { Outlet } from "react-router";
import { NotFound } from "@/app/not-found";
import { usePrincipal } from "./session";

// Only an administrator has the administration screens; the address answers as an unknown one for anyone else.
export function RequireAdministrator() {
  const principal = usePrincipal();
  return principal?.role === "administrator" ? <Outlet /> : <NotFound />;
}
