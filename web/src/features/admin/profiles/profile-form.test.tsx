import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { adminProfile, installMockApi, signedIn } from "../../../../tests/msw";
import { location, renderApp } from "../../../../tests/render";

const mock = installMockApi(signedIn());

const production = adminProfile({
  revision: 3,
  credential_file: "production-token",
  allowed_namespaces: ["payments", "billing"],
  default_timezone: "America/Sao_Paulo",
});

beforeEach(() => {
  window.localStorage.clear();
  mock.session = signedIn();
  mock.adminProfiles = [production];
});

function changes() {
  return mock.calls.filter((call) => call.method !== "GET");
}

async function fillNew() {
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("Identifier"), "lab");
  await user.type(screen.getByLabelText("Display name"), "Lab");
  await user.type(screen.getByLabelText("Endpoint URL"), "https://lab.example.com");
  await user.type(screen.getByLabelText("Cluster name"), "lab-1");
  await user.type(screen.getByLabelText("Allowed namespaces"), "payments");
  return user;
}

describe("the profile form", () => {
  it("lays out a new profile", async () => {
    await renderApp("/profiles/new");
    await screen.findByRole("heading", { level: 1, name: "New profile" });
    const headings = screen.getAllByRole("heading", { level: 2 }).map((heading) => heading.textContent);
    expect(headings).toEqual(["Identity", "Metric source", "Cluster", "Namespaces", "Report form choices", "Thresholds"]);
    expect(screen.getByLabelText("Identifier")).toHaveClass("font-mono");
    const change = screen.getAllByRole("button", { name: "Change" });
    expect(change.map((button) => button.getAttribute("aria-expanded"))).toEqual(["false", "false"]);
    expect(screen.getByText("Any period · 24h · America/Sao_Paulo · Detailed · All")).toHaveClass("font-mono");
    expect(
      screen.getByText(
        "Attention 70 % · Critical 85 % · Low utilization 30 % · Near maximum 80 % · Node pool spread 30 · Variation 100 % · Divergence 1",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Saving creates revision 1.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Cancel" })).toHaveAttribute("href", "/profiles");
    expect(screen.getByLabelText("Allowed namespaces")).toHaveAttribute("rows", "5");
  });

  it("opens and folds a section", async () => {
    await renderApp("/profiles/new");
    const user = userEvent.setup();
    const [first] = await screen.findAllByRole("button", { name: "Change" });
    await user.click(first as HTMLElement);
    expect(first).toHaveAccessibleName("Fold");
    expect(first).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByLabelText("Permitted periods")).toBeVisible();
    await user.click(first as HTMLElement);
    expect(screen.getByLabelText("Permitted periods")).not.toBeVisible();
  });

  it("creates a profile", async () => {
    const { router } = await renderApp("/profiles/new");
    const user = await fillNew();
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(location(router)).toBe("/profiles?open=lab");
    });
    expect(changes()).toHaveLength(1);
    expect(changes()[0]).toMatchObject({ method: "POST", path: "/admin/profiles" });
    expect(changes()[0]?.body).toEqual({
      id: "lab",
      display_name: "Lab",
      active: false,
      endpoint_url: "https://lab.example.com",
      endpoint_type: "auto",
      timeout_seconds: 90,
      tls_verify: true,
      credential_file: "",
      ca_bundle_file: "",
      cluster_name: "lab-1",
      cluster_label: "cluster",
      filter_mode: "required",
      nodepool_label: "",
      node_autoscaling: "auto",
      allowed_namespaces: ["payments"],
      excluded_containers: [],
      permitted_periods: [],
      default_period: null,
      permitted_timezones: [],
      default_timezone: null,
      permitted_report_profiles: [],
      default_report_profile: null,
      permitted_min_severities: [],
      default_min_severity: null,
      thresholds: {
        warning_percent: 70,
        critical_percent: 85,
        low_utilization_percent: 30,
        autoscaler_near_max_percent: 80,
        nodepool_spread_points: 30,
        bursty_gap_percent: 100,
        hpa_replica_divergence: 1,
      },
    });
    expect(await screen.findByRole("status")).toHaveTextContent("The profile Lab was created.");
  });

  it("sends the ticked choices", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    await user.click(screen.getAllByRole("button", { name: "Change" })[0] as HTMLElement);
    await user.click(screen.getByLabelText("Executive"));
    await user.type(screen.getByLabelText("Permitted periods"), "24h{enter}7d");
    await user.selectOptions(screen.getByLabelText("Default report profile"), "Executive");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(changes()).toHaveLength(1);
    });
    expect(changes()[0]?.body).toMatchObject({
      permitted_report_profiles: ["executive"],
      default_report_profile: "executive",
      permitted_periods: ["24h", "7d"],
      default_period: null,
    });
  });

  it("refuses a form that fails a check and sends nothing", async () => {
    await renderApp("/profiles/new");
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Save" }));
    expect(await screen.findAllByText("Fill in this field.")).toHaveLength(4);
    expect(screen.getByText("Give at least one namespace.")).toBeInTheDocument();
    expect(screen.getByLabelText("Identifier")).toHaveFocus();
    expect(changes()).toEqual([]);
  });

  it("opens the folded section that holds a failed check", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    const [choices] = screen.getAllByRole("button", { name: "Change" });
    await user.click(choices as HTMLElement);
    await user.type(screen.getByLabelText("Permitted periods"), "24h");
    await user.type(screen.getByLabelText("Default period"), "7d");
    await user.click(choices as HTMLElement);
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Use one of the permitted values.")).toBeVisible();
    expect(screen.getByLabelText("Default period")).toHaveFocus();
    expect(changes()).toEqual([]);
  });

  it("shows an edit with its revision", async () => {
    await renderApp("/profiles/production/edit");
    await screen.findByRole("heading", { level: 1, name: "Production" });
    expect(screen.getByText("Profile")).toBeInTheDocument();
    expect(screen.getByText("production")).toHaveClass("font-mono");
    expect(screen.getByText(/Revision 3, saved /)).toBeInTheDocument();
    expect(screen.queryByLabelText("Identifier")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Display name")).toHaveValue("Production");
    expect(screen.getByLabelText("Credential file")).toHaveValue("production-token");
    expect(screen.getByLabelText("Allowed namespaces")).toHaveValue("payments\nbilling");
    expect(
      screen.getByText("Saving writes revision 4. Reports already requested keep the revision they were requested with."),
    ).toBeInTheDocument();
  });

  it("saves the whole document of an edit", async () => {
    const { router } = await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    const name = await screen.findByLabelText("Display name");
    await user.clear(name);
    await user.type(name, "Production 2");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(location(router)).toBe("/profiles?open=production");
    });
    expect(changes()).toHaveLength(1);
    const call = changes()[0];
    expect(call).toMatchObject({ method: "PATCH", path: "/admin/profiles/production" });
    expect(Object.keys(call?.body as object).sort()).toEqual(
      [
        "base_revision_id",
        ...Object.keys(production).filter((key) => !["id", "revision_id", "revision", "created_at"].includes(key)),
      ].sort(),
    );
    expect(call?.body).toMatchObject({ base_revision_id: production.revision_id, display_name: "Production 2" });
    expect(await screen.findByRole("status")).toHaveTextContent("The profile Production 2 was saved.");
  });

  it("opens the folded sections that hold a refused field", async () => {
    await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await screen.findByRole("heading", { level: 1, name: "Production" });
    mock.failures.set("PATCH /admin/profiles/production", {
      status: 422,
      code: "PROFILE_DOCUMENT_INVALID",
      fields: { default_timezone: "invalid", thresholds: "invalid" },
    });
    await user.click(screen.getByRole("button", { name: "Save" }));
    const fold = await screen.findAllByRole("button", { name: "Fold" });
    expect(fold.map((button) => button.getAttribute("aria-expanded"))).toEqual(["true", "true"]);
    expect(screen.getAllByText("This value is not valid.")).toHaveLength(2);
    expect(screen.getByLabelText("Default time zone")).toHaveFocus();
    expect(screen.getByLabelText("Default time zone")).toHaveAccessibleDescription("This value is not valid.");
    expect(screen.getByLabelText("Attention from (%)")).toHaveAccessibleDescription("This value is not valid.");
  });

  it("shows a refusal of a field the form lacks in the alert", async () => {
    await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await screen.findByRole("heading", { level: 1, name: "Production" });
    mock.failures.set("PATCH /admin/profiles/production", {
      status: 422,
      code: "PROFILE_DOCUMENT_INVALID",
      fields: { nope: "invalid" },
    });
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("The profile is not valid.");
  });

  it("puts a refused address under the endpoint", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    mock.failures.set("POST /admin/profiles", { status: 422, code: "ENDPOINT_ADDRESS_REJECTED" });
    await user.click(screen.getByRole("button", { name: "Save" }));
    const sentence = "The metric source resolves to an address that is not allowed.";
    await waitFor(() => {
      expect(screen.getByLabelText("Endpoint URL")).toHaveAccessibleDescription(sentence);
    });
    expect(screen.getByLabelText("Endpoint URL")).toHaveFocus();
  });

  it("puts a taken identifier under the identifier", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    mock.failures.set("POST /admin/profiles", { status: 409, code: "PROFILE_ID_TAKEN" });
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(screen.getByLabelText("Identifier")).toHaveAccessibleDescription(/This profile id is already taken\./);
    });
  });

  it("shows any other failure above the save bar", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    mock.failures.set("POST /admin/profiles", { status: 500, code: "INTERNAL_ERROR" });
    await user.click(screen.getByRole("button", { name: "Save" }));
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText(/./)).toBeInTheDocument();
  });

  it("cancels without asking", async () => {
    const { router } = await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    const name = await screen.findByLabelText("Display name");
    await user.type(name, "x");
    await user.click(screen.getByRole("link", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await waitFor(() => {
      expect(location(router)).toBe("/profiles");
    });
    expect(changes()).toEqual([]);
  });

  it("says when the profile is not found", async () => {
    await renderApp("/profiles/nope/edit");
    expect(await screen.findByText("No such profile was found.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to the profiles" })).toHaveAttribute("href", "/profiles");
  });
});
