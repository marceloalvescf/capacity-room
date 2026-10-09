import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import { adminUser, installMockApi, principal, signedIn } from "../../../../tests/msw";
import { location, renderApp } from "../../../../tests/render";

const mock = installMockApi(signedIn({ username: "admin" }));

const self = adminUser({
  id: principal().id,
  username: "admin",
  role: "administrator",
  language: "en",
});
const ana = adminUser();
const carla = adminUser({
  id: "a1b2c3d4-0000-4000-8000-000000000002",
  username: "carla.lima",
  role: "reader",
  active: false,
});
const diego = adminUser({
  id: "a1b2c3d4-0000-4000-8000-000000000003",
  username: "diego",
  role: "administrator",
  language: "en",
});

beforeEach(() => {
  window.localStorage.clear();
  mock.session = signedIn({ username: "admin" });
  mock.adminUsers = [self, ana, carla, diego];
});

async function openRow(name: string) {
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: `Show the details of ${name}` }));
  return user;
}

// The summary row of a user, the one with the button that opens it.
function rowOf(name: string): HTMLElement {
  const row = screen.getByRole("button", { name: new RegExp(`the details of ${name}$`) }).closest("tr");
  if (row === null) {
    throw new Error("no row");
  }
  return row;
}

function changes() {
  return mock.calls.filter((call) => call.method !== "GET");
}

describe("the users screen", () => {
  it("lists the users", async () => {
    mock.adminUsers = [diego, carla, self, ana];
    await renderApp("/users");
    await screen.findByRole("table", { name: "Users" });
    const rows = within(screen.getByRole("table")).getAllByRole("row").slice(1);
    expect(rows.map((row) => within(row).getAllByRole("cell")[1]?.textContent)).toEqual([
      "diego",
      "carla.lima",
      "admin (you)",
      "ana.souza",
    ]);
    expect(screen.getByText("(you)")).toHaveClass("text-muted-fg");
    expect(screen.getByText("Inactive")).toHaveClass("text-muted-fg");
    expect(screen.getAllByText("Active")).toHaveLength(3);
  });

  it("only an administrator", async () => {
    for (const role of ["operator", "reader"] as const) {
      mock.session = signedIn({ role });
      const { unmount } = await renderApp("/users");
      expect(await screen.findByRole("heading", { name: "Page not found" })).toBeInTheDocument();
      expect(within(screen.getByRole("navigation", { name: "Main navigation" })).queryByRole("link", { name: "Users" })).toBeNull();
      unmount();
    }
    mock.session = signedIn();
    await renderApp("/reports");
    const nav = await screen.findByRole("navigation", { name: "Main navigation" });
    expect(within(nav).getAllByRole("link").map((link) => link.textContent)).toEqual([
      "Reports",
      "New report",
      "Users",
      "Account",
    ]);
  });

  it("save waits for a change", async () => {
    await renderApp("/users");
    const user = await openRow("ana.souza");
    const save = screen.getByRole("button", { name: "Save changes" });
    expect(save).toBeDisabled();
    await user.selectOptions(screen.getByLabelText("Role"), "reader");
    expect(save).toBeEnabled();
  });

  it("changes role and language", async () => {
    await renderApp("/users");
    const user = await openRow("ana.souza");
    await user.selectOptions(screen.getByLabelText("Role"), "reader");
    await user.selectOptions(screen.getByLabelText("Language"), "en");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await screen.findByText("Saved. The user was signed out everywhere.")).toBeInTheDocument();
    expect(changes().map((call) => [call.method, call.path, call.body])).toEqual([
      ["PATCH", `/admin/users/${ana.id}`, { role: "reader", language: "en" }],
    ]);
    await waitFor(() => {
      expect(within(rowOf("ana.souza")).getByRole("cell", { name: "Reader" })).toBeInTheDocument();
    });
  });

  it("language alone", async () => {
    await renderApp("/users");
    const user = await openRow("ana.souza");
    await user.selectOptions(screen.getByLabelText("Language"), "en");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await screen.findByText("Saved.")).toBeInTheDocument();
    expect(changes().map((call) => call.body)).toEqual([{ language: "en" }]);
  });

  it("sets a password", async () => {
    await renderApp("/users");
    const user = await openRow("ana.souza");
    const field = screen.getByLabelText("New password");
    expect(field).toHaveAttribute("type", "password");
    expect(field).toHaveAttribute("autocomplete", "new-password");
    await user.type(field, "12345678901");
    await user.click(screen.getByRole("button", { name: "Set password" }));
    expect(await screen.findByText("Use at least 12 characters.")).toBeInTheDocument();
    expect(changes()).toEqual([]);
    await user.type(field, "2");
    await user.click(screen.getByRole("button", { name: "Set password" }));
    expect(await screen.findByText("Password set. The user was signed out everywhere.")).toBeInTheDocument();
    expect(changes().map((call) => call.body)).toEqual([{ password: "123456789012" }]);
    expect(field).toHaveValue("");
  });

  it("deactivates after asking", async () => {
    await renderApp("/users");
    const user = await openRow("ana.souza");
    await user.click(screen.getByRole("button", { name: "Deactivate" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Deactivate ana.souza?" });
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(changes()).toEqual([]);
    await user.click(screen.getByRole("button", { name: "Deactivate" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Deactivate" }));
    expect(await screen.findByRole("button", { name: "Reactivate" })).toBeInTheDocument();
    expect(changes().map((call) => [call.method, call.path])).toEqual([["DELETE", `/admin/users/${ana.id}`]]);
    expect(within(rowOf("ana.souza")).getByRole("cell", { name: "Inactive" })).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("ana.souza was deactivated.");
  });

  it("reactivates", async () => {
    await renderApp("/users");
    const user = await openRow("carla.lima");
    await user.click(screen.getByRole("button", { name: "Reactivate" }));
    await waitFor(() => {
      expect(screen.getAllByText("Active")).toHaveLength(4);
    });
    expect(changes().map((call) => [call.method, call.body])).toEqual([["PATCH", { active: true }]]);
    expect(screen.getByRole("status")).toHaveTextContent("carla.lima can sign in again.");
  });

  it("last administrator", async () => {
    mock.failures.set(`PATCH /admin/users/${diego.id}`, { status: 409, code: "LAST_ADMIN" });
    await renderApp("/users");
    const user = await openRow("diego");
    await user.selectOptions(screen.getByLabelText("Role"), "reader");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(i18next.t("error.LAST_ADMIN"));
    expect(alert.closest("section")).toContainElement(screen.getByRole("button", { name: "Save changes" }));
  });

  it("last administrator on deactivate", async () => {
    mock.failures.set(`DELETE /admin/users/${diego.id}`, { status: 409, code: "LAST_ADMIN" });
    await renderApp("/users");
    const user = await openRow("diego");
    await user.click(screen.getByRole("button", { name: "Deactivate" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Deactivate" }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(i18next.t("error.LAST_ADMIN"));
    expect(alert.closest("section")).toContainElement(screen.getByRole("button", { name: "Deactivate" }));
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });

  it("a notice goes when a value of its part changes", async () => {
    await renderApp("/users");
    const user = await openRow("ana.souza");
    await user.selectOptions(screen.getByLabelText("Language"), "en");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await screen.findByText("Saved.");
    await user.selectOptions(screen.getByLabelText("Language"), "pt-BR");
    expect(screen.queryByText("Saved.")).toBeNull();
    await user.type(screen.getByLabelText("New password"), "123456789012");
    await user.click(screen.getByRole("button", { name: "Set password" }));
    await screen.findByText("Password set. The user was signed out everywhere.");
    await user.type(screen.getByLabelText("New password"), "x");
    expect(screen.queryByText("Password set. The user was signed out everywhere.")).toBeNull();
  });

  it("says when the users cannot be loaded", async () => {
    mock.failures.set("GET /admin/users", { status: 500, code: "INTERNAL_ERROR" });
    await renderApp("/users");
    expect(await screen.findByRole("alert")).toHaveTextContent("The users could not be loaded.");
  });

  it("says when there is no user", async () => {
    mock.adminUsers = [];
    await renderApp("/users");
    expect(await screen.findByText("No user yet.")).toBeInTheDocument();
  });

  it("own row", async () => {
    await renderApp("/users");
    await openRow("admin");
    expect(await screen.findByText("Created 2026-09-01")).toBeInTheDocument();
    expect(screen.getByText(i18next.t("admin.users.own"))).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open the account page" })).toHaveAttribute("href", "/account");
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.queryByLabelText("New password")).toBeNull();
    expect(screen.queryByRole("button", { name: "Deactivate" })).toBeNull();
  });

  it("creates a user", async () => {
    const { router } = await renderApp("/users");
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "New user" }));
    const dialog = await screen.findByRole("dialog", { name: "New user" });
    await user.click(within(dialog).getByRole("button", { name: "Create user" }));
    expect(await within(dialog).findAllByText("Fill in this field.")).toHaveLength(2);
    expect(changes()).toEqual([]);
    await user.type(within(dialog).getByLabelText("User name"), "bruno");
    await user.type(within(dialog).getByLabelText("Password"), "a-long-secret");
    await user.selectOptions(within(dialog).getByLabelText("Role"), "operator");
    await user.selectOptions(within(dialog).getByLabelText("Language"), "pt-BR");
    await user.click(within(dialog).getByRole("button", { name: "Create user" }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
    expect(changes().map((call) => call.body)).toEqual([
      { username: "bruno", password: "a-long-secret", role: "operator", language: "pt-BR" },
    ]);
    const created = mock.adminUsers.find((candidate) => candidate.username === "bruno");
    expect(location(router)).toBe(`/users?open=${created?.id ?? ""}`);
    expect(await screen.findByRole("button", { name: "Hide the details of bruno" })).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("bruno was created.");
  });

  it("leaves role and language at their defaults", async () => {
    await renderApp("/users");
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "New user" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("User name"), "bruno");
    await user.type(within(dialog).getByLabelText("Password"), "a-long-secret");
    await user.click(within(dialog).getByRole("button", { name: "Create user" }));
    await waitFor(() => {
      expect(changes()).toHaveLength(1);
    });
    expect(changes().map((call) => call.body)).toEqual([
      { username: "bruno", password: "a-long-secret", role: "reader", language: "en" },
    ]);
  });

  it("taken name", async () => {
    mock.failures.set("POST /admin/users", { status: 409, code: "USERNAME_TAKEN" });
    await renderApp("/users");
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "New user" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("User name"), "ana.souza");
    await user.type(within(dialog).getByLabelText("Password"), "a-long-secret");
    await user.click(within(dialog).getByRole("button", { name: "Create user" }));
    const sentence = await within(dialog).findByText(i18next.t("error.USERNAME_TAKEN"));
    const name = within(dialog).getByLabelText("User name");
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(name.getAttribute("aria-describedby")?.split(" ")).toContain(sentence.id);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("escape closes the dialog", async () => {
    await renderApp("/users");
    const user = userEvent.setup();
    const opener = await screen.findByRole("button", { name: "New user" });
    await user.click(opener);
    await screen.findByRole("dialog");
    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
    await waitFor(() => {
      expect(opener).toHaveFocus();
    });
  });
});
