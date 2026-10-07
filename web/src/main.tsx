import "@/lib/zod";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "@/app/app";
import "@/i18n/i18n";
import "@/theme/fonts.css";
import "@/theme/theme.css";

const root = document.getElementById("root");
if (root === null) {
  throw new Error("missing root element");
}

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
