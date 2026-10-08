export const reportTabs = [
  "overview",
  "findings",
  "namespaces",
  "workloads",
  "autoscaling",
  "nodes",
  "quality",
] as const;

export type ReportTab = (typeof reportTabs)[number];
