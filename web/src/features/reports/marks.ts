import type { components } from "@/api/schema.gen";
import type { Status } from "@/components/status-mark";

type Schemas = components["schemas"];

export function reportMark(report: { status: Schemas["ReportStatus"]; expired: boolean }): Status {
  if (report.expired) {
    return "unavailable";
  }
  switch (report.status) {
    case "queued":
    case "running":
      return "undefined";
    case "collected":
      return "healthy";
    case "failed":
      return "critical";
  }
}

export function pdfMark(status: Schemas["PDFStatus"]): Status {
  switch (status) {
    case "pending":
      return "undefined";
    case "ready":
      return "healthy";
    case "failed":
      return "critical";
  }
}
