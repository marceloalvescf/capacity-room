import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { adminProfile, anonymous, installMockApi, signedIn } from "../../../../tests/msw";
import { location, renderApp, sentence } from "../../../../tests/render";
import { findShown, shown } from "../../../../tests/shown";

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
    for (const name of ["Change Report form choices", "Change Thresholds"]) {
      expect(screen.getByRole("button", { name })).toHaveAttribute("aria-expanded", "false");
      expect(screen.getByRole("button", { name })).toHaveTextContent(/^Change$/);
    }
    expect(screen.getByLabelText("Active")).toHaveAccessibleDescription("An active profile is offered in the new report form.");
    const choices = screen.getByText("Any period").closest("p") as HTMLElement;
    expect(choices).toHaveTextContent(/^Any period · 24h · America\/Sao_Paulo · Detailed · All$/);
    expect(choices).not.toHaveClass("font-mono");
    expect(within(choices).getByText("America/Sao_Paulo")).toHaveClass("font-mono");
    const thresholds = screen.getByText("Low utilization").closest("p") as HTMLElement;
    expect(thresholds).toHaveTextContent(
      /^Attention 70% · Critical 85% · Low utilization 30% · Near maximum 80% · Node pool spread 30 · Variation 100% · Divergence 1$/,
    );
    expect(within(thresholds).getByText("70%")).toHaveClass("font-mono");
    expect(screen.getByText("Saving creates revision 1.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Cancel" })).toHaveAttribute("href", "/profiles");
    expect(screen.getByLabelText("Allowed namespaces")).toHaveAttribute("rows", "5");
  });

  it("opens and folds a section", async () => {
    await renderApp("/profiles/new");
    const user = userEvent.setup();
    const first = await screen.findByRole("button", { name: "Change Report form choices" });
    await user.click(first);
    expect(first).toHaveAccessibleName("Fold Report form choices");
    expect(first).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByLabelText("Permitted periods")).toBeVisible();
    await user.click(first);
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
    await user.click(screen.getByRole("button", { name: "Change Report form choices" }));
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
    const choices = screen.getByRole("button", { name: "Change Report form choices" });
    await user.click(choices);
    await user.type(screen.getByLabelText("Permitted periods"), "24h");
    await user.type(screen.getByLabelText("Default period"), "7d");
    await user.click(screen.getByRole("button", { name: "Fold Report form choices" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Use one of the permitted values.")).toBeVisible();
    expect(screen.getByLabelText("Default period")).toHaveFocus();
    expect(changes()).toEqual([]);
  });

  it("shows an edit with its revision", async () => {
    await renderApp("/profiles/production/edit");
    const heading = await screen.findByRole("heading", { level: 1, name: "Production" });
    expect(heading.previousElementSibling).toBeNull();
    expect(screen.getByText("production")).toHaveClass("font-mono");
    expect(heading.nextElementSibling).toHaveTextContent(/^Profile · production · Revision 3, saved /);
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
    for (const name of ["Fold Report form choices", "Fold Thresholds"]) {
      expect(await screen.findByRole("button", { name })).toHaveAttribute("aria-expanded", "true");
    }
    expect(screen.getAllByText("This value is not valid.")).toHaveLength(2);
    expect(screen.getByLabelText("Default time zone")).toHaveFocus();
    expect(screen.getByLabelText("Default time zone")).toHaveAccessibleDescription("This value is not valid.");
    expect(screen.getByLabelText("Attention from (%)")).toHaveAccessibleDescription("This value is not valid.");
  });

  it("takes the thresholds mark off when any threshold changes", async () => {
    await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await screen.findByRole("heading", { level: 1, name: "Production" });
    mock.failures.set("PATCH /admin/profiles/production", {
      status: 422,
      code: "PROFILE_DOCUMENT_INVALID",
      fields: { thresholds: "incoherent" },
    });
    await user.click(screen.getByRole("button", { name: "Save" }));
    const warning = await screen.findByLabelText("Attention from (%)");
    await waitFor(() => {
      expect(warning).toHaveAttribute("aria-invalid", "true");
    });
    await user.type(screen.getByLabelText("Critical from (%)"), "1");
    expect(warning).not.toHaveAttribute("aria-invalid", "true");
  });

  it("refuses a threshold above 100 in the browser", async () => {
    await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Change Thresholds" }));
    const field = screen.getByLabelText("Critical from (%)");
    await user.clear(field);
    await user.type(field, "101");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Use a number from 0 to 100.")).toBeInTheDocument();
    expect(changes()).toEqual([]);
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

  it("words a key the form lacks beside a key that landed", async () => {
    await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await screen.findByRole("heading", { level: 1, name: "Production" });
    mock.failures.set("PATCH /admin/profiles/production", {
      status: 422,
      code: "PROFILE_DOCUMENT_INVALID",
      fields: { default_timezone: "invalid", nope: "invalid" },
    });
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("The profile is not valid.");
    expect(screen.getByLabelText("Default time zone")).toHaveAccessibleDescription("This value is not valid.");
  });

  it("shows a refusal on a checkbox, a select, and a list of checkboxes", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    mock.failures.set("POST /admin/profiles", {
      status: 422,
      code: "PROFILE_DOCUMENT_INVALID",
      fields: { permitted_report_profiles: "empty", filter_mode: "invalid", active: "incoherent", tls_verify: "invalid" },
    });
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(screen.getByLabelText("Active")).toHaveFocus();
    });
    expect(screen.getByLabelText("Active")).toHaveAccessibleDescription(
      "An active profile is offered in the new report form. This value does not agree with another field.",
    );
    expect(screen.getByLabelText("Verify the TLS certificate")).toHaveAccessibleDescription("This value is not valid.");
    expect(screen.getByLabelText("Filter by cluster")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Filter by cluster")).toHaveAccessibleDescription("This value is not valid.");
    expect(screen.getByLabelText("Executive")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Executive")).toHaveAccessibleDescription(
      "None ticked: any. Give at least one value.",
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("words a refused identifier on an edit in the alert", async () => {
    await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await screen.findByRole("heading", { level: 1, name: "Production" });
    mock.failures.set("PATCH /admin/profiles/production", {
      status: 422,
      code: "PROFILE_DOCUMENT_INVALID",
      fields: { id: "invalid" },
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

  it("tests the draft", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    const line = await findShown("Connected. The source answers every query the report needs.");
    expect(line.parentElement?.querySelector("svg")).toHaveClass("text-healthy");
    expect(line.parentElement?.querySelector("circle")).not.toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent("Connected. The source answers every query the report needs.");
    const tests = mock.calls.filter((call) => call.path === "/admin/profiles/test");
    expect(tests).toHaveLength(1);
    expect(tests[0]).toMatchObject({ method: "POST" });
    expect(tests[0]?.body).toMatchObject({
      id: "lab",
      display_name: "Lab",
      endpoint_url: "https://lab.example.com",
      cluster_name: "lab-1",
      allowed_namespaces: ["payments"],
    });
    expect(changes().filter((call) => call.path === "/admin/profiles")).toEqual([]);
  });

  it("puts the test in the metric source section, not the save bar", async () => {
    await renderApp("/profiles/new");
    const test = await screen.findByRole("button", { name: "Test connection" });
    const section = screen.getByRole("heading", { level: 2, name: "Metric source" }).closest("section");
    expect(section).toContainElement(test);
    const save = screen.getByRole("button", { name: "Save" });
    expect(save.parentElement).not.toContainElement(test);
  });

  it("tests without the browser's checks", async () => {
    await renderApp("/profiles/new");
    const user = userEvent.setup();
    mock.profileTests.set("draft", { ok: false, code: "ENDPOINT_UNREACHABLE", capabilities: [] });
    await user.click(await screen.findByRole("button", { name: "Test connection" }));
    expect(await findShown("The source could not be reached, or did not answer.")).toBeInTheDocument();
    expect(mock.calls.filter((call) => call.path === "/admin/profiles/test")).toHaveLength(1);
    expect(screen.queryByText("Fill in this field.")).not.toBeInTheDocument();
  });

  it("refuses a draft", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    mock.failures.set("POST /admin/profiles/test", { status: 422, code: "ENDPOINT_ADDRESS_REJECTED" });
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    expect(await findShown("Correct the marked fields, then test again.")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Correct the marked fields, then test again.");
    expect(screen.getByLabelText("Endpoint URL")).toHaveAccessibleDescription(
      "The metric source resolves to an address that is not allowed.",
    );
  });

  it("takes a refusal off a field when its value changes", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    mock.failures.set("POST /admin/profiles/test", { status: 422, code: "ENDPOINT_ADDRESS_REJECTED" });
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    await waitFor(() => {
      expect(screen.getByLabelText("Endpoint URL")).toHaveAttribute("aria-invalid", "true");
    });
    await user.type(screen.getByLabelText("Endpoint URL"), "/x");
    expect(screen.getByLabelText("Endpoint URL")).not.toHaveAttribute("aria-invalid", "true");
    expect(screen.queryByText("The metric source resolves to an address that is not allowed.")).not.toBeInTheDocument();
  });

  it("takes every earlier refusal off when a new test answers", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    mock.failures.set("POST /admin/profiles/test", { status: 422, code: "ENDPOINT_ADDRESS_REJECTED" });
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    await waitFor(() => {
      expect(screen.getByLabelText("Endpoint URL")).toHaveAttribute("aria-invalid", "true");
    });
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    await findShown("Connected. The source answers every query the report needs.");
    expect(screen.getByLabelText("Endpoint URL")).not.toHaveAttribute("aria-invalid", "true");
    expect(screen.queryByText("The metric source resolves to an address that is not allowed.")).not.toBeInTheDocument();
  });

  it("shows the failure of a draft in a 200", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    mock.profileTests.set("draft", { ok: false, code: "ENDPOINT_UNREACHABLE", capabilities: [] });
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    const line = await findShown("The source could not be reached, or did not answer.");
    expect(line.parentElement?.querySelector("svg")).toHaveClass("text-critical");
    expect(line.parentElement?.querySelector("polygon")).not.toBeNull();
  });

  it("clears the result on a change", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    await findShown("Connected. The source answers every query the report needs.");
    await user.type(screen.getByLabelText("Cluster name"), "x");
    expect(shown("Connected. The source answers every query the report needs.")).toEqual([]);
  });

  it("clears the result on a select and a checkbox", async () => {
    await renderApp("/profiles/new");
    const user = await fillNew();
    const connected = "Connected. The source answers every query the report needs.";
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    await findShown(connected);
    await user.selectOptions(screen.getByLabelText("Type"), "Prometheus");
    expect(shown(connected)).toEqual([]);
    await user.click(screen.getByRole("button", { name: "Test connection" }));
    await findShown(connected);
    await user.click(screen.getByLabelText("Active"));
    expect(shown(connected)).toEqual([]);
  });

  it("ignores a refusal that arrives after a change", async () => {
    await renderApp("/profiles/new");
    await fillNew();
    mock.failures.set("POST /admin/profiles/test", { status: 422, code: "ENDPOINT_ADDRESS_REJECTED" });
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));
    fireEvent.change(screen.getByLabelText("Cluster name"), { target: { value: "other" } });
    await waitFor(() => {
      expect(mock.calls.some((call) => call.path === "/admin/profiles/test")).toBe(true);
    });
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(screen.getByLabelText("Endpoint URL")).not.toHaveAttribute("aria-invalid", "true");
    expect(shown("Correct the marked fields, then test again.")).toEqual([]);
  });

  async function staleSave() {
    const view = await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    const name = await screen.findByLabelText("Display name");
    await user.clear(name);
    await user.type(name, "Mine");
    mock.failures.set("PATCH /admin/profiles/production", { status: 409, code: "PROFILE_REVISION_STALE" });
    await user.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByText(/Someone saved this profile after you opened it\./);
    return { ...view, user };
  }

  it("keeps the values on a stale revision", async () => {
    await staleSave();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Someone saved this profile after you opened it. Your values are kept here.",
    );
    expect(screen.getByRole("button", { name: "Load the latest revision" })).toBeInTheDocument();
    expect(screen.getByLabelText("Display name")).toHaveValue("Mine");
  });

  it("asks before loading the latest", async () => {
    const { user } = await staleSave();
    mock.adminProfiles = [
      { ...production, revision: 4, revision_id: "7b1e5c0a-0000-4000-8000-000000000004", display_name: "Theirs" },
    ];
    mock.calls.length = 0;
    await user.click(screen.getByRole("button", { name: "Load the latest revision" }));
    expect(await screen.findByRole("alertdialog", { name: "Leave without saving?" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Stay" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(mock.calls).toEqual([]);
    expect(screen.getByLabelText("Display name")).toHaveValue("Mine");
    await user.click(screen.getByRole("button", { name: "Load the latest revision" }));
    await user.click(await screen.findByRole("button", { name: "Leave" }));
    await waitFor(() => {
      expect(screen.getByLabelText("Display name")).toHaveValue("Theirs");
    });
    expect(mock.calls.map((call) => `${call.method} ${call.path}`)).toEqual(["GET /admin/profiles/production"]);
    expect(screen.getByText(sentence(/Revision 4, saved /))).toBeInTheDocument();
    expect(screen.getByText(/Saving writes revision 5\./)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("resets the form when the latest revision is the one held", async () => {
    const { user } = await staleSave();
    await user.click(screen.getByRole("button", { name: "Load the latest revision" }));
    await user.click(await screen.findByRole("button", { name: "Leave" }));
    await waitFor(() => {
      expect(screen.getByLabelText("Display name")).toHaveValue("Production");
    });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("keeps the guard after a Cancel click that opens another tab", async () => {
    await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText("Display name"), "x");
    await user.keyboard("{Control>}");
    await user.click(screen.getByRole("link", { name: "Cancel" }));
    await user.keyboard("{/Control}");
    await user.click(screen.getByRole("link", { name: "Reports" }));
    expect(await screen.findByRole("alertdialog", { name: "Leave without saving?" })).toBeInTheDocument();
    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(true);
  });

  it("guards a leave inside the app", async () => {
    const { router } = await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText("Display name"), "x");
    await user.click(screen.getByRole("link", { name: "Reports" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Leave without saving?" });
    expect(within(dialog).getByText("The changes to this profile will be lost.")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Stay" })).toHaveFocus();
    await user.click(within(dialog).getByRole("button", { name: "Stay" }));
    expect(location(router)).toBe("/profiles/production/edit");
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("link", { name: "Reports" }));
    await user.click(await screen.findByRole("button", { name: "Leave" }));
    await waitFor(() => {
      expect(location(router)).toBe("/reports");
    });
  });

  it("lets a session that ended through", async () => {
    const { router } = await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText("Display name"), "x");
    mock.failures.set("PATCH /admin/profiles/production", { status: 401, code: "UNAUTHENTICATED" });
    mock.session = anonymous();
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(location(router)).toContain("/login?next=");
    });
    expect(location(router)).toContain("ended=1");
    expect(screen.queryByRole("alertdialog", { name: "Leave without saving?" })).not.toBeInTheDocument();
    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(false);
  });

  it("lets a sign-out through", async () => {
    const { router } = await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText("Display name"), "x");
    await user.click(within(screen.getByRole("banner")).getByRole("button", { name: "Sign out" }));
    await waitFor(() => {
      expect(location(router)).toBe("/login");
    });
    expect(screen.queryByRole("alertdialog", { name: "Leave without saving?" })).not.toBeInTheDocument();
  });

  it("does not guard a clean or a saving form", async () => {
    const { router } = await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    await screen.findByLabelText("Display name");
    await user.click(screen.getByRole("link", { name: "Reports" }));
    await waitFor(() => {
      expect(location(router)).toBe("/reports");
    });
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    await router.navigate("/profiles/production/edit");
    await user.type(await screen.findByLabelText("Display name"), "x");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(location(router)).toBe("/profiles?open=production");
    });
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("stops the browser's unload while changed", async () => {
    await renderApp("/profiles/production/edit");
    const user = userEvent.setup();
    const name = await screen.findByLabelText("Display name");
    const clean = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(clean);
    expect(clean.defaultPrevented).toBe(false);
    await user.type(name, "x");
    const changed = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(changed);
    expect(changed.defaultPrevented).toBe(true);
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => {
      expect(screen.queryByLabelText("Display name")).not.toBeInTheDocument();
    });
    const saved = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(saved);
    expect(saved.defaultPrevented).toBe(false);
  });

  it("says when the profile is not found", async () => {
    await renderApp("/profiles/nope/edit");
    expect(await screen.findByText("No such profile was found.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to the profiles" })).toHaveAttribute("href", "/profiles");
  });
});
