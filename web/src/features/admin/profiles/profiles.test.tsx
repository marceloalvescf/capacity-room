import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import { adminProfile, installMockApi, profileSummary, server, signedIn } from "../../../../tests/msw";
import { location, renderApp } from "../../../../tests/render";
import { findShown, shown } from "../../../../tests/shown";

const mock = installMockApi(signedIn());

const dr = adminProfile({
  id: "dr",
  revision_id: "5d1f3f0e-1b7d-4a53-8f55-0c9c5d0a0002",
  display_name: "Disaster recovery",
  cluster_name: "dr-1",
});
const lab = adminProfile({
  id: "lab",
  revision_id: "5d1f3f0e-1b7d-4a53-8f55-0c9c5d0a0003",
  display_name: "Lab",
  active: false,
  cluster_name: "lab-1",
});
const production = adminProfile({
  revision: 3,
  credential_file: "production-token",
  cluster_name: "prod-1",
  allowed_namespaces: ["payments", "billing"],
  default_period: "24h",
  default_timezone: "America/Sao_Paulo",
  default_report_profile: "detailed",
  default_min_severity: "all",
});
const staging = adminProfile({
  id: "staging",
  revision_id: "5d1f3f0e-1b7d-4a53-8f55-0c9c5d0a0004",
  display_name: "Staging",
  cluster_name: "stg-1",
});

beforeEach(() => {
  window.localStorage.clear();
  mock.session = signedIn();
  mock.adminProfiles = [dr, lab, production, staging];
});

async function openRow(name: string) {
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: `Show the details of ${name}` }));
  return user;
}

function changes() {
  return mock.calls.filter((call) => call.method !== "GET");
}

describe("the profiles screen", () => {
  it("lists the profiles", async () => {
    await renderApp("/profiles");
    await screen.findByRole("table", { name: "Profiles" });
    const rows = within(screen.getByRole("table")).getAllByRole("row").slice(1);
    expect(rows.map((row) => within(row).getAllByRole("cell").map((cell) => cell.textContent))).toEqual([
      ["", "Disaster recoverydr", "dr-1", "Active"],
      ["", "Lablab", "lab-1", "Inactive"],
      ["", "Productionproduction", "prod-1", "Active"],
      ["", "Stagingstaging", "stg-1", "Active"],
    ]);
    expect(screen.getByText("production")).toHaveClass("font-mono", "text-muted-fg");
    expect(screen.getByText("prod-1")).toHaveClass("font-mono");
    expect(screen.getByText("Inactive")).toHaveClass("text-muted-fg");
    expect(screen.getByRole("link", { name: "New profile" })).toHaveAttribute("href", "/profiles/new");
  });

  it("opens a profile", async () => {
    const { router } = await renderApp("/profiles");
    await openRow("production");
    expect(location(router)).toBe("/profiles?open=production");
    const fact = (label: string) => screen.getByText(label).closest("div");
    expect(fact("Endpoint")).toHaveTextContent("https://prometheus.example.com");
    expect(within(fact("Endpoint") as HTMLElement).getByText("https://prometheus.example.com")).toHaveClass("font-mono");
    expect(fact("Type")).toHaveTextContent("Detect");
    expect(within(fact("Credential file") as HTMLElement).getByText("production-token")).toHaveClass("font-mono");
    expect(within(fact("CA bundle file") as HTMLElement).getByText("None")).toHaveClass("text-muted-fg");
    const revision = fact("Revision") as HTMLElement;
    expect(revision).toHaveTextContent("3, saved 2026-09-01");
    expect(revision.querySelector(".font-mono")).toBeNull();
    expect(within(fact("Namespaces") as HTMLElement).getByText("payments, billing")).toHaveClass("font-mono");
    const choices = fact("Report form choices") as HTMLElement;
    expect(choices).toHaveTextContent("Any period · 24h · America/Sao_Paulo · Detailed · All");
    expect(within(choices).getByText("Any period")).not.toHaveClass("font-mono");
    expect(within(choices).getByText("America/Sao_Paulo")).toHaveClass("font-mono");
    expect(screen.getByRole("link", { name: "Edit" })).toHaveAttribute("href", "/profiles/production/edit");
  });

  it("tests the saved revision", async () => {
    mock.profileTests.set("staging", { ok: false, code: "ENDPOINT_AUTH_REJECTED", capabilities: [] });
    await renderApp("/profiles");
    const user = await openRow("production");
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    const connected = await findShown("Connected. The source answers every query the report needs.");
    expect(connected.parentElement?.querySelector("circle")).not.toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent("Connected. The source answers every query the report needs.");
    expect(changes().map((call) => [call.method, call.path])).toEqual([["POST", "/admin/profiles/production/test"]]);

    await user.click(screen.getByRole("button", { name: "Hide the details of production" }));
    await user.click(screen.getByRole("button", { name: "Show the details of staging" }));
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    const refused = await findShown("The source refused the credential.");
    expect(refused.parentElement?.querySelector("polygon")).not.toBeNull();

    await user.click(screen.getByRole("button", { name: "Hide the details of staging" }));
    await user.click(screen.getByRole("button", { name: "Show the details of production" }));
    expect(shown("Connected. The source answers every query the report needs.")).toHaveLength(1);
  });

  it("the test button waits for the answer", async () => {
    let answer: () => void = () => undefined;
    const answered = new Promise<void>((resolve) => {
      answer = resolve;
    });
    server.use(
      http.post(`${window.location.origin}/api/v1/admin/profiles/production/test`, async () => {
        await answered;
        return HttpResponse.json({ ok: true, code: "PROFILE_OK", capabilities: [] });
      }),
    );
    await renderApp("/profiles");
    const user = await openRow("production");
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    expect(await screen.findByRole("button", { name: "Testing…" })).toBeDisabled();
    answer();
    expect(await screen.findByRole("button", { name: "Test connection" })).toBeEnabled();
  });

  it("test fails", async () => {
    mock.failures.set("POST /admin/profiles/production/test", { status: 500, code: "INTERNAL_ERROR" });
    await renderApp("/profiles");
    const user = await openRow("production");
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    const sentence = await findShown(i18next.t("error.INTERNAL_ERROR"));
    expect(sentence.parentElement?.querySelector("polygon")).not.toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent(i18next.t("error.INTERNAL_ERROR"));
  });

  it("deletes after asking", async () => {
    mock.profiles = [profileSummary()];
    const { router } = await renderApp("/generate");
    const user = userEvent.setup();
    await screen.findByRole("heading", { level: 1, name: "New report" });
    await user.click(screen.getByRole("link", { name: "Profiles" }));
    await user.click(await screen.findByRole("button", { name: "Show the details of lab" }));
    expect(screen.getByRole("button", { name: "Delete" })).not.toHaveClass("bg-critical");
    await user.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Delete the profile Lab?" });
    expect(within(dialog).getByRole("button", { name: "Delete" })).toHaveClass("bg-critical");
    expect(dialog).toHaveTextContent(
      "It leaves this list and the new report form. Reports already requested keep it, and its identifier can be used again.",
    );
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(changes()).toEqual([]);
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Delete" })).toHaveFocus();
    });
    const summaryFetches = () => mock.calls.filter((call) => call.method === "GET" && call.path === "/profiles").length;
    expect(summaryFetches()).toBe(1);
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => {
      expect(screen.queryByRole("button", { name: "Show the details of lab" })).toBeNull();
    });
    expect(changes().map((call) => [call.method, call.path])).toEqual([["DELETE", "/admin/profiles/lab"]]);
    expect(location(router)).toBe("/profiles");
    expect(screen.getByRole("status")).toHaveTextContent("The profile Lab was deleted.");
    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1, name: "Profiles" })).toHaveFocus();
    });
    expect(screen.getByRole("heading", { level: 1, name: "Profiles" })).toHaveAttribute("tabindex", "-1");
    expect(mock.adminProfiles.map((profile) => profile.id)).toEqual(["dr", "production", "staging"]);
    await user.click(screen.getByRole("link", { name: "New report" }));
    await waitFor(() => {
      expect(summaryFetches()).toBe(2);
    });
  });

  it("a failed delete shows above the table", async () => {
    mock.failures.set("DELETE /admin/profiles/lab", { status: 500, code: "INTERNAL_ERROR" });
    await renderApp("/profiles");
    const user = await openRow("lab");
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(i18next.t("error.INTERNAL_ERROR"));
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.getByRole("button", { name: "Hide the details of lab" })).toBeInTheDocument();
  });

  it("empty list", async () => {
    mock.adminProfiles = [];
    await renderApp("/profiles");
    expect(await screen.findByText("No profile yet.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "New profile" })).toBeInTheDocument();
  });

  it("says when the profiles cannot be loaded", async () => {
    mock.failures.set("GET /admin/profiles", { status: 500, code: "INTERNAL_ERROR" });
    await renderApp("/profiles");
    expect(await screen.findByRole("alert")).toHaveTextContent("The profiles could not be loaded.");
  });

  it("navigation", async () => {
    await renderApp("/reports");
    const nav = await screen.findByRole("navigation", { name: "Main navigation" });
    expect(within(nav).getAllByRole("link").map((link) => link.textContent)).toEqual([
      "Home",
      "Reports",
      "New report",
      "Profiles",
      "Users",
      "Account",
    ]);
  });

  it("only an administrator", async () => {
    mock.session = signedIn({ role: "operator" });
    await renderApp("/profiles");
    expect(await screen.findByRole("heading", { name: "Page not found" })).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "Main navigation" })).queryByRole("link", { name: "Profiles" })).toBeNull();
  });
});

describe("the mock of the profile routes", () => {
  const send = async (method: string, path: string, body: unknown) => {
    const response = await fetch(`${window.location.origin}/api/v1${path}`, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return (await response.json()) as ReturnType<typeof adminProfile>;
  };

  it("fills what a create leaves out and keeps the thresholds left out", async () => {
    mock.adminProfiles = [];
    const created = await send("POST", "/admin/profiles", { id: "new", thresholds: { warning_percent: 60 } });
    expect(created).toMatchObject({
      id: "new",
      active: false,
      timeout_seconds: 90,
      tls_verify: true,
      cluster_label: "cluster",
      default_period: null,
      revision: 1,
    });
    expect(created.thresholds).toEqual({ ...adminProfile().thresholds, warning_percent: 60 });
  });

  it("raises the revision only when a revise changes something", async () => {
    mock.adminProfiles = [production];
    const same = await send("PATCH", "/admin/profiles/production", {
      base_revision_id: production.revision_id,
      display_name: "Production",
      thresholds: { warning_percent: 70 },
    });
    expect(same.revision).toBe(3);
    expect(same.revision_id).toBe(production.revision_id);
    const changed = await send("PATCH", "/admin/profiles/production", {
      base_revision_id: production.revision_id,
      thresholds: { critical_percent: 90 },
    });
    expect(changed.revision).toBe(4);
    expect(changed.thresholds).toEqual({ ...production.thresholds, critical_percent: 90 });
  });

  it("lists in identifier order", async () => {
    mock.adminProfiles = [staging, production, dr];
    const response = await fetch(`${window.location.origin}/api/v1/admin/profiles`);
    const body = (await response.json()) as { profiles: { id: string }[] };
    expect(body.profiles.map((profile) => profile.id)).toEqual(["dr", "production", "staging"]);
  });
});
