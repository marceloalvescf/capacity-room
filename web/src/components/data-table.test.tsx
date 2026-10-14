import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RouterProvider, createMemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { LiveRegion } from "@/app/live-region";
import "@/i18n/i18n";
import { DataTable, type Column } from "./data-table";
import * as reveal from "./reveal";
import { withTooltips } from "../../tests/render";

interface Row {
  key: string;
  name: string;
  status: string;
  cpu: number | null;
  pods: number;
}

const columns: Column<Row>[] = [
  { id: "name", header: "Name", cell: (row) => row.name, sortValue: (row) => row.name },
  { id: "cpu", header: "CPU", cell: (row) => (row.cpu === null ? "n/a" : String(row.cpu)), sortValue: (row) => row.cpu, align: "end" },
  { id: "pods", header: "Pods", cell: (row) => String(row.pods), sortValue: (row) => row.pods, align: "end" },
  { id: "plain", header: "Plain", cell: () => "x" },
];

const mixed: Row[] = [
  { key: "healthy-b", name: "b", status: "healthy", cpu: 3, pods: 1 },
  { key: "critical-c", name: "c", status: "critical", cpu: null, pods: 2 },
  { key: "attention-a", name: "a", status: "attention", cpu: 1, pods: 3 },
  { key: "critical-a", name: "a", status: "critical", cpu: 2, pods: 4 },
];

const named: Row[] = [
  { key: "web-1", name: "payments-web", status: "healthy", cpu: 1, pods: 1 },
  { key: "api-1", name: "api-gateway", status: "healthy", cpu: 2, pods: 2 },
  { key: "db-1", name: "db", status: "healthy", cpu: 3, pods: 3 },
];

// A latency makes every change of the address land late, as a router that works in a transition does.
function mount(rows: Row[], path = "/", defaultOrder: "status" | "view" = "status", latency = 0, withSearch = true) {
  const router = createMemoryRouter(
    [
      {
        path: "*",
        ...(latency === 0 ? {} : { loader: () => new Promise((resolve) => setTimeout(resolve, latency)) }),
        hydrateFallbackElement: <div aria-busy="true" />,
        element: (
          <DataTable
            label="Things"
            rows={rows}
            rowKey={(row) => row.key}
            rowName={(row) => row.name}
            columns={columns}
            empty="No rows in this report."
            {...(withSearch ? { search: { label: "Search things", text: (row: Row) => row.name } } : {})}
            renderOpened={(row) => <p>{`details of ${row.key}`}</p>}
            defaultOrder={defaultOrder}
            status={(row) => row.status}
          />
        ),
      },
    ],
    { initialEntries: [path] },
  );
  render(
    withTooltips(<LiveRegion>
      <RouterProvider router={router} />
    </LiveRegion>),
  );
  return router;
}

function order(): string[] {
  const rows = within(screen.getByRole("table", { name: "Things" })).getAllByRole("row").slice(1);
  return rows.filter((row) => within(row).getAllByRole("cell").length > 1).map((row) => within(row).getAllByRole("cell")[1]?.textContent ?? "");
}

function search(router: ReturnType<typeof mount>): string {
  return router.state.location.search;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("data table", () => {
  it("default order", () => {
    mount(mixed);
    const rows = within(screen.getByRole("table", { name: "Things" })).getAllByRole("row").slice(1);
    expect(rows.map((row) => within(row).getAllByRole("cell").slice(1, 3).map((cell) => cell.textContent).join("|"))).toEqual([
      "a|2",
      "c|n/a",
      "a|1",
      "b|3",
    ]);
  });

  it("the view's order", () => {
    mount(mixed, "/", "view");
    expect(order()).toEqual(["b", "c", "a", "a"]);
  });

  it("sort by a column", async () => {
    const router = mount(mixed);
    const header = screen.getByRole("columnheader", { name: "CPU" });
    expect(header).not.toHaveAttribute("aria-sort");
    await userEvent.click(within(header).getByRole("button", { name: "CPU" }));
    expect(header).toHaveAttribute("aria-sort", "ascending");
    expect(search(router)).toBe("?sort=cpu.asc");
    expect(order()).toEqual(["a", "a", "b", "c"]);
    await userEvent.click(within(header).getByRole("button", { name: "CPU" }));
    expect(header).toHaveAttribute("aria-sort", "descending");
    expect(search(router)).toBe("?sort=cpu.desc");
    expect(order()).toEqual(["b", "a", "a", "c"]);
    expect(within(screen.getByRole("columnheader", { name: "Plain" })).queryByRole("button")).not.toBeInTheDocument();
  });

  it("nulls sort last", async () => {
    mount(mixed, "/?sort=cpu.asc");
    const cells = () =>
      within(screen.getByRole("table", { name: "Things" }))
        .getAllByRole("row")
        .slice(1)
        .map((row) => within(row).getAllByRole("cell")[2] as HTMLElement);
    expect(cells().map((cell) => cell.textContent)).toEqual(["1", "2", "3", "n/a"]);
    await userEvent.click(screen.getByRole("button", { name: "CPU" }));
    expect(cells().map((cell) => cell.textContent)).toEqual(["3", "2", "1", "n/a"]);
  });

  it("search", async () => {
    const router = mount(named);
    await userEvent.type(screen.getByRole("searchbox", { name: "Search things" }), "WEB");
    expect(order()).toEqual(["payments-web"]);
    expect(screen.getByText("Rows: 1", { selector: "p" })).toBeInTheDocument();
    expect(search(router)).toBe("?q=WEB");
  });

  it("no match and empty table", async () => {
    mount(named);
    expect(screen.getByText("Rows: 3")).toBeInTheDocument();
    await userEvent.type(screen.getByRole("searchbox", { name: "Search things" }), "zzz");
    expect(screen.getByText("No row matches the search.")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("empty table", () => {
    mount([]);
    expect(screen.getByText("No rows in this report.")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("open one row", async () => {
    const router = mount(named, "/", "view");
    const first = screen.getByRole("button", { name: "Show the details of payments-web" });
    expect(first).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(first);
    expect(screen.getByRole("button", { name: "Hide the details of payments-web" })).toHaveAttribute("aria-expanded", "true");
    expect(search(router)).toBe("?open=web-1");
    const opened = screen.getByText("details of web-1").closest("td") as HTMLElement;
    expect(opened).toHaveAttribute("colspan", String(columns.length + 1));
    expect(opened.closest("tr")).toHaveClass("bg-bg");
    expect(screen.getByRole("button", { name: "Hide the details of payments-web" }).closest("tr")).toHaveClass("bg-info-surface");
    await userEvent.click(screen.getByRole("button", { name: "Show the details of api-gateway" }));
    expect(screen.getAllByText(/^details of /)).toHaveLength(1);
    expect(screen.getByText("details of api-1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show the details of payments-web" })).toHaveAttribute("aria-expanded", "false");
    expect(search(router)).toBe("?open=api-1");
    await userEvent.click(screen.getByRole("button", { name: "Hide the details of api-gateway" }));
    expect(search(router)).toBe("");
  });

  it("stale address", () => {
    mount(mixed, "/?open=nope&sort=nope.up&q=");
    expect(order()).toEqual(["a", "c", "a", "b"]);
    expect(screen.queryByText(/^details of /)).not.toBeInTheDocument();
    expect(screen.getByText("Rows: 4")).toBeInTheDocument();
  });

  it("address restores", () => {
    mount(named, "/?q=a&sort=pods.desc&open=api-1");
    expect(screen.getByRole("searchbox", { name: "Search things" })).toHaveValue("a");
    expect(order()).toEqual(["api-gateway", "payments-web"]);
    expect(screen.getByText("details of api-1")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Pods" })).toHaveAttribute("aria-sort", "descending");
  });

  it("keeps the typed text when a sort follows at once", async () => {
    const router = mount(named, "/", "status", 40);
    const box = await screen.findByRole("searchbox", { name: "Search things" });
    fireEvent.change(box, { target: { value: "p" } });
    fireEvent.change(box, { target: { value: "pay" } });
    fireEvent.click(screen.getByRole("button", { name: "Name" }));
    await waitFor(() => {
      expect(search(router)).toBe("?q=pay&sort=name.asc");
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 100));
    });
    expect(box).toHaveValue("pay");
    expect(order()).toEqual(["payments-web"]);
  });

  it("follows a change of the address from outside", async () => {
    const router = mount(named, "/?q=api");
    const box = screen.getByRole("searchbox", { name: "Search things" });
    expect(box).toHaveValue("api");
    await act(async () => {
      await router.navigate("/?q=db");
    });
    expect(box).toHaveValue("db");
    expect(order()).toEqual(["db"]);
    await act(async () => {
      await router.navigate("/");
    });
    expect(box).toHaveValue("");
    expect(order()).toHaveLength(3);
  });

  it("names the column of the expanders", () => {
    mount(named);
    const header = screen.getByRole("columnheader", { name: "Details" });
    expect(header).toHaveClass("w-12");
    expect(within(header).getByText("Details")).toHaveClass("sr-only");
  });

  it("sorts by one column at a time", async () => {
    const router = mount(mixed);
    await userEvent.click(screen.getByRole("button", { name: "CPU" }));
    fireEvent.click(screen.getByRole("button", { name: "Pods" }), { shiftKey: true });
    await waitFor(() => {
      expect(search(router)).toBe("?sort=pods.asc");
    });
  });

  it("no search", () => {
    mount(named, "/?q=web", "status", 0, false);
    expect(screen.queryByRole("searchbox")).not.toBeInTheDocument();
    expect(screen.queryByText(/^Rows: /)).not.toBeInTheDocument();
    expect(order()).toHaveLength(3);
  });

  it("third click removes the sort", async () => {
    const router = mount(mixed);
    const header = screen.getByRole("columnheader", { name: "CPU" });
    const button = within(header).getByRole("button", { name: "CPU" });
    await userEvent.click(button);
    await userEvent.click(button);
    expect(search(router)).toBe("?sort=cpu.desc");
    await userEvent.click(button);
    expect(search(router)).toBe("");
    expect(header).not.toHaveAttribute("aria-sort");
    expect(order()).toEqual(["a", "c", "a", "b"]);
  });

  it("announces the count of a search", async () => {
    mount(named);
    const region = screen.getByRole("status");
    await act(async () => {
      await Promise.resolve();
    });
    expect(region).toBeEmptyDOMElement();
    await userEvent.type(screen.getByRole("searchbox", { name: "Search things" }), "web");
    await waitFor(() => {
      expect(region).toHaveTextContent("Rows: 1");
    });
  });

  it("opening reveals the row", async () => {
    const spy = vi.spyOn(reveal, "revealRows").mockImplementation(() => undefined);
    mount(named, "/", "view");
    await userEvent.click(screen.getByRole("button", { name: "Show the details of payments-web" }));
    expect(spy).toHaveBeenCalledTimes(1);
    const [row, opened, head] = spy.mock.calls[0] as [HTMLElement, HTMLElement, number];
    expect(row).toBe(screen.getByRole("button", { name: "Hide the details of payments-web" }).closest("tr"));
    expect(opened).toBe(screen.getByText("details of web-1").closest("tr"));
    expect(head).toBe(document.querySelector("thead")?.getBoundingClientRect().height);
    await userEvent.click(screen.getByRole("button", { name: "Hide the details of payments-web" }));
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it("reveals a row opened by the address on load", () => {
    const spy = vi.spyOn(reveal, "revealRows").mockImplementation(() => undefined);
    mount(named, "/?open=api-1", "view");
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy.mock.calls[0]?.[1]).toBe(screen.getByText("details of api-1").closest("tr"));
  });

  it("reveals a row opened by the address when a search stops hiding it", async () => {
    const spy = vi.spyOn(reveal, "revealRows").mockImplementation(() => undefined);
    mount(named, "/?open=api-1&q=zzz", "view");
    expect(spy).not.toHaveBeenCalled();
    await userEvent.clear(screen.getByRole("searchbox", { name: "Search things" }));
    await waitFor(() => {
      expect(spy).toHaveBeenCalledTimes(1);
    });
    expect(spy.mock.calls[0]?.[1]).toBe(screen.getByText("details of api-1").closest("tr"));
  });

  it("every header wears the label", () => {
    mount(named);
    for (const header of screen.getAllByRole("columnheader").slice(1)) {
      expect(header).toHaveClass("label");
    }
  });

  it("a column gives a cell the classes of its row", () => {
    const tinted: Column<Row>[] = [
      ...columns,
      { id: "tint", header: "Tint", cell: (row) => row.key, cellClassName: (row) => (row.pods > 1 ? "bg-depth-2" : "") },
    ];
    render(
      withTooltips(<LiveRegion>
        <RouterProvider
          router={createMemoryRouter(
            [
              {
                path: "*",
                element: (
                  <DataTable
                    label="Things"
                    rows={named}
                    rowKey={(row) => row.key}
                    rowName={(row) => row.name}
                    columns={tinted}
                    empty="No rows in this report."
                    renderOpened={() => null}
                  />
                ),
              },
            ],
            { initialEntries: ["/"] },
          )}
        />
      </LiveRegion>),
    );
    expect(screen.getByRole("cell", { name: "web-1" })).not.toHaveClass("bg-depth-2");
    expect(screen.getByRole("cell", { name: "api-1" })).toHaveClass("bg-depth-2");
  });

  it("a cell breaks no word; an opened row breaks only a value longer than the table", () => {
    mount(named, "/?open=web-1");
    const cells = screen.getAllByRole("cell");
    expect(cells.length).toBeGreaterThan(0);
    for (const cell of cells) {
      expect(cell).not.toHaveClass("[overflow-wrap:anywhere]");
    }
    expect(screen.getByText("details of web-1").parentElement).toHaveClass("[overflow-wrap:anywhere]");
  });
});
