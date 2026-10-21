import { act, cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { components } from "@/api/schema.gen";
import { i18next } from "@/i18n/i18n";
import { installMockApi, profileSummary, report, signedIn } from "../../../tests/msw";
import { location, renderApp } from "../../../tests/render";
import { mockColourScheme } from "../../../tests/match-media";
import viewEn from "../../../tests/fixtures/view/edge-scaling.en.json";
import viewPt from "../../../tests/fixtures/view/edge-scaling.pt-BR.json";

const mock = installMockApi(signedIn());

type View = components["schemas"]["ReportView"];

const id = "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01";
const english = viewEn as View;
const portuguese = viewPt as View;

beforeEach(() => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.profiles = [profileSummary()];
  mock.reports = [report()];
  mock.views.set(`${id}:en`, english);
  mock.views.set(`${id}:pt-BR`, portuguese);
});

function groupButtons(): HTMLElement[] {
  return screen.getAllByRole("heading", { level: 2 }).flatMap((heading) => within(heading).queryAllByRole("button"));
}

async function open(search = "", language: "en" | "pt-BR" = "en", fold = false) {
  const view = await renderApp(`/reports/${id}/findings${search}`, language);
  await screen.findByRole("searchbox", { name: i18next.t("report.findings.search") });
  if (!fold) {
    for (const button of groupButtons().filter((candidate) => candidate.getAttribute("aria-expanded") === "false")) {
      await userEvent.click(button);
    }
  }
  return view;
}

function params(router: Awaited<ReturnType<typeof open>>["router"]): URLSearchParams {
  return new URLSearchParams(router.state.location.search);
}

function chips(): HTMLElement[] {
  return within(screen.getByRole("group", { name: "Severity" })).getAllByRole("button");
}

function chip(name: string): HTMLElement {
  return within(screen.getByRole("group", { name: "Severity" })).getByRole("button", { name: new RegExp(name) });
}

function items(): HTMLElement[] {
  return screen
    .getAllByRole("list")
    .flatMap((list) => within(list).queryAllByRole("button"))
    .filter((button) => button.hasAttribute("aria-pressed"));
}

function shownSentences(): string[] {
  return items().map((button) => button.textContent);
}

function detail(): HTMLElement {
  return screen.getByRole("article");
}

const sentenceOf = (index: number) => english.findings[index]?.sentence ?? "";

describe("findings tab", () => {
  it("grouped, critical first", async () => {
    await open();
    const headings = screen.getAllByRole("heading", { level: 2 }).filter((h) => /^(Critical|Attention|Healthy)/.test(h.textContent));
    expect(headings.map((h) => h.textContent)).toEqual(["Critical10", "Attention11", "Healthy7"]);
    const lists = headings.map((h) => (h.parentElement as HTMLElement).querySelector("ul") as HTMLElement);
    const critical = within(lists[0] as HTMLElement).getAllByRole("button");
    expect(critical).toHaveLength(10);
    expect(critical[0]).toHaveTextContent(sentenceOf(0));
    expect(critical[9]).toHaveTextContent(sentenceOf(9));
    expect(within(lists[2] as HTMLElement).getAllByRole("button")[0]).toHaveTextContent(sentenceOf(21));
    expect(chips().map((c) => c.textContent)).toEqual(["Critical10", "Attention11", "Healthy7"]);
    expect(within(chip("Critical")).getByText("10")).toHaveClass("font-mono");
  });

  it("chips", async () => {
    const user = userEvent.setup();
    const { router } = await open();
    await user.click(chip("Healthy"));
    expect(chip("Healthy")).toHaveAttribute("aria-pressed", "false");
    expect(chip("Critical")).toHaveAttribute("aria-pressed", "true");
    expect(shownSentences().join()).not.toContain(sentenceOf(21));
    expect(items()).toHaveLength(21);
    expect(params(router).get("severity")).toBe("critical,attention");
    await user.click(chip("Healthy"));
    expect(params(router).has("severity")).toBe(false);
  });

  it("see findings arrives", async () => {
    await open("?severity=critical");
    expect(chips().map((c) => c.getAttribute("aria-pressed"))).toEqual(["true", "false", "false"]);
    expect(items()).toHaveLength(10);
  });

  it("scope select", async () => {
    const user = userEvent.setup();
    const { router } = await open();
    const select = screen.getByRole("combobox", { name: "Scope" });
    expect(within(select).getAllByRole("option").map((o) => o.textContent)).toEqual([
      "All scopes",
      "Namespace",
      "Workload",
      "HPA",
      "Node",
      "Node pool",
      "Cluster",
    ]);
    await user.selectOptions(select, "Node");
    expect(params(router).get("scope")).toBe("node");
    expect(items()).toHaveLength(11);
    for (const button of items()) {
      expect(button).toHaveTextContent("Node");
    }
  });

  it("search", async () => {
    const user = userEvent.setup();
    const { router } = await open();
    await user.type(screen.getByRole("searchbox"), "PAYMENTS/IDLE");
    expect(params(router).get("q")).toBe("PAYMENTS/IDLE");
    expect(items()).toHaveLength(1);
    expect(items()[0]).toHaveTextContent(sentenceOf(12));
    await user.clear(screen.getByRole("searchbox"));
    await user.type(screen.getByRole("searchbox"), "consolidation");
    expect(items()).toHaveLength(1);
    expect(items()[0]).toHaveTextContent(sentenceOf(16));
    await user.clear(screen.getByRole("searchbox"));
    await user.type(screen.getByRole("searchbox"), "against the limit");
    expect(items()).toHaveLength(4);
  });

  it("keeps the typed search through another change and follows the address", async () => {
    const user = userEvent.setup();
    const { router } = await open();
    const box = screen.getByRole("searchbox");
    await user.type(box, "increased");
    await user.click(chip("Healthy"));
    expect(box).toHaveValue("increased");
    expect(params(router).get("q")).toBe("increased");
    expect(params(router).get("severity")).toBe("critical,attention");
    await act(async () => {
      await router.navigate(`/reports/${id}/findings?q=consolidation`);
    });
    expect(box).toHaveValue("consolidation");
    expect(items()).toHaveLength(1);
  });

  it("the search reads the shown scope name, the sentence and the action", async () => {
    const quiet = english.findings.map((finding, index) =>
      index === 10 ? { ...finding, action: "Review the headroom." } : finding,
    );
    mock.views.set(`${id}:en`, { ...english, findings: quiet });
    const user = userEvent.setup();
    await open();
    await user.type(screen.getByRole("searchbox"), "cluster");
    expect(items()).toHaveLength(0);
    await user.clear(screen.getByRole("searchbox"));
    await user.type(screen.getByRole("searchbox"), "Undetermined");
    expect(items()).toHaveLength(1);
    expect(items()[0]).toHaveTextContent(sentenceOf(20));
    await user.clear(screen.getByRole("searchbox"));
    await user.type(screen.getByRole("searchbox"), "Review the headroom");
    expect(items()).toHaveLength(1);
  });

  it("the detail shows the kind of a workload beside its scope", async () => {
    await open("?finding=6");
    expect(within(detail()).getByText("Workload · Deployment")).toBeInTheDocument();
    cleanup();
    await open("?finding=2");
    expect(within(detail()).getByText("Namespace")).toBeInTheDocument();
    expect(detail().querySelector("header")).not.toHaveTextContent("·");
  });

  it("an empty severity shows no finding", async () => {
    await open("?severity=");
    expect(chips().map((c) => c.getAttribute("aria-pressed"))).toEqual(["false", "false", "false"]);
    expect(items()).toHaveLength(0);
    expect(screen.getByText("No finding matches these filters.")).toBeInTheDocument();
  });

  it("a severity list with a stranger presses the known one alone", async () => {
    await open("?severity=critical,bogus");
    expect(chips().map((c) => c.getAttribute("aria-pressed"))).toEqual(["true", "false", "false"]);
    expect(items()).toHaveLength(10);
  });

  it("a stale scope is ignored", async () => {
    await open("?scope=bogus");
    expect(screen.getByRole("combobox", { name: "Scope" })).toHaveValue("");
    expect(items()).toHaveLength(28);
  });

  it("a finding the filters hide selects nothing", async () => {
    await open("?finding=19&severity=critical");
    expect(items()).toHaveLength(10);
    expect(items().every((button) => button.getAttribute("aria-pressed") === "false")).toBe(true);
    expect(screen.queryByRole("article")).toBeNull();
    expect(screen.getByText(i18next.t("report.findings.choose"))).toBeInTheDocument();
  });

  it("opens folded with a prompt and no finding selected", async () => {
    const { router } = await open("", "en", true);
    expect(groupButtons().map((button) => button.getAttribute("aria-expanded"))).toEqual(["false", "false", "false"]);
    expect(items()).toHaveLength(0);
    expect(screen.queryByRole("article")).toBeNull();
    expect(screen.getByText("Choose a finding to see what was observed, what to do, and its evidence.")).toBeInTheDocument();
    expect(router.state.location.search).toBe("");
  });

  it("a group opens and folds, several at once, and an item opens its detail", async () => {
    const user = userEvent.setup();
    const { router } = await open("", "en", true);
    const [critical, attention] = groupButtons() as [HTMLElement, HTMLElement];
    await user.click(critical);
    await user.click(attention);
    expect(critical).toHaveAttribute("aria-expanded", "true");
    expect(items()).toHaveLength(21);
    expect(document.getElementById(critical.getAttribute("aria-controls") ?? "")).toHaveClass("max-h-[26rem]");
    await user.click(critical);
    expect(items()).toHaveLength(11);
    expect(router.state.location.search).toBe("");
    await user.click(items()[0] as HTMLElement);
    expect(params(router).get("finding")).toBe("10");
    expect(within(detail()).getByText(sentenceOf(10))).toBeInTheDocument();
  });

  it("a finding named in the address opens its group alone", async () => {
    await open("?finding=12", "en", true);
    expect(groupButtons().map((button) => button.getAttribute("aria-expanded"))).toEqual(["false", "true", "false"]);
    expect(items()).toHaveLength(11);
    expect(within(detail()).getByText(sentenceOf(12))).toBeInTheDocument();
  });

  it("a search opens the groups that hold a match and a group can still be folded", async () => {
    const user = userEvent.setup();
    await open("", "en", true);
    await user.type(screen.getByRole("searchbox"), "against the limit");
    expect(items()).toHaveLength(4);
    const opened = groupButtons().filter((button) => button.getAttribute("aria-expanded") === "true");
    expect(opened.length).toBeGreaterThan(0);
    await user.click(opened[0] as HTMLElement);
    expect(opened[0]).toHaveAttribute("aria-expanded", "false");
  });

  it("a scope and severity filter in the address opens the groups that still show a finding", async () => {
    await open("?scope=node&severity=critical", "en", true);
    expect(groupButtons().map((button) => button.getAttribute("aria-expanded"))).toEqual(["true"]);
    expect(items().length).toBeGreaterThan(0);
  });

  it("a severity filter alone opens its group, and a group can still be folded", async () => {
    const user = userEvent.setup();
    await open("?severity=attention", "en", true);
    expect(groupButtons().map((button) => button.getAttribute("aria-expanded"))).toEqual(["true"]);
    expect(items()).toHaveLength(11);
    await user.click(groupButtons()[0] as HTMLElement);
    expect(items()).toHaveLength(0);
  });

  it("a finding chosen through the address opens its group again after the reader folded it", async () => {
    const user = userEvent.setup();
    const { router } = await open("?finding=12", "en", true);
    await user.click(groupButtons()[1] as HTMLElement);
    expect(items()).toHaveLength(0);
    await act(async () => {
      await router.navigate(`/reports/${id}/findings?finding=13`);
    });
    expect(groupButtons().map((button) => button.getAttribute("aria-expanded"))).toEqual(["false", "true", "false"]);
    expect(items().filter((button) => button.getAttribute("aria-pressed") === "true")).toHaveLength(1);
    expect(within(detail()).getByText(sentenceOf(13))).toBeInTheDocument();
  });

  it("scrolls a finding named in the address into view once", async () => {
    const scroll = vi.spyOn(Element.prototype, "scrollIntoView");
    const user = userEvent.setup();
    await open("?finding=5");
    await waitFor(() => {
      expect(scroll).toHaveBeenCalledTimes(1);
    });
    const entry = scroll.mock.contexts[0] as HTMLElement;
    expect(entry).toHaveAttribute("aria-pressed", "true");
    expect(entry).toHaveTextContent(sentenceOf(5));
    await user.click(items()[0] as HTMLElement);
    expect(scroll).toHaveBeenCalledTimes(1);
  });

  it("a row shows the shape of its severity and names it to screen readers alone", async () => {
    await open();
    const row = items()[0] as HTMLElement;
    const word = within(row).getByText("Critical");
    expect(word).toHaveClass("sr-only");
    expect(row.querySelector("svg")).toHaveClass("text-critical");
  });

  it("select one", async () => {
    const user = userEvent.setup();
    const { router } = await open();
    const third = items()[3] as HTMLElement;
    await user.click(third);
    expect(third).toHaveAttribute("aria-pressed", "true");
    expect(items()[0]).toHaveAttribute("aria-pressed", "false");
    expect(params(router).get("finding")).toBe("3");
    const finding = english.findings[3];
    const article = within(detail());
    expect(article.getByRole("heading", { name: "payments" })).toHaveClass("font-mono");
    expect(article.getByText("Namespace")).toBeInTheDocument();
    expect(article.getByText("Confidence: Inferred. Deduced from approximate or partial evidence.")).toBeInTheDocument();
    expect(article.getByText("Observed")).toBeInTheDocument();
    expect(article.getByText(finding?.sentence ?? "")).toBeInTheDocument();
    expect(article.getByText("What to do")).toBeInTheDocument();
    expect(article.getByText(finding?.action ?? "")).toBeInTheDocument();
    expect(article.getByText("Evidence")).toBeInTheDocument();
    expect(detail().querySelector("dl")).toBeNull();
    expect(article.getByText("660%")).toHaveClass("font-mono");
    expect(article.getByText("440%")).toHaveClass("font-mono");
    expect(detail().querySelector("svg[viewBox='0 0 1000 16']")).not.toBeNull();
    expect(document.body.textContent).not.toContain(finding?.code ?? "?");
  });

  it("evidence the proof does not show is listed beneath it", async () => {
    await open("?finding=12");
    const finding = english.findings[12];
    const terms = detail().querySelectorAll("dt");
    expect(terms).toHaveLength(1);
    const first = finding?.evidence[0];
    expect(terms[0]).toHaveTextContent(i18next.t(`evidence.${first?.name ?? "peak"}`));
    expect(within(terms[0]?.parentElement as HTMLElement).getByText(first?.text ?? "")).toHaveClass("font-mono");
    expect(detail().querySelector("svg[viewBox='0 0 1000 16']")).toBeNull();
  });

  it("stale finding", async () => {
    const { router } = await open("?finding=99&severity=bogus");
    expect(chips().map((c) => c.getAttribute("aria-pressed"))).toEqual(["true", "true", "true"]);
    expect(items()).toHaveLength(28);
    expect(screen.queryByRole("article")).toBeNull();
    expect(location(router)).toBe(`/reports/${id}/findings?finding=99&severity=bogus`);
  });

  it("no findings", async () => {
    mock.views.set(`${id}:en`, { ...english, findings: [] });
    await renderApp(`/reports/${id}/findings`, "en");
    expect(await screen.findByText("This report holds no findings.")).toBeInTheDocument();
    expect(screen.queryByRole("article")).toBeNull();
  });

  it("no match", async () => {
    const user = userEvent.setup();
    await open();
    for (const name of ["Critical", "Attention", "Healthy"]) {
      await user.click(chip(name));
    }
    expect(screen.getByText("No finding matches these filters.")).toBeInTheDocument();
    expect(screen.queryByRole("article")).toBeNull();
    expect(chips().map((c) => c.getAttribute("aria-pressed"))).toEqual(["false", "false", "false"]);
  });

  it("show all findings clears the severities, the scope and the search", async () => {
    const user = userEvent.setup();
    const { router } = await open("?severity=&scope=node&q=nothing");
    expect(screen.getByText("No finding matches these filters.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Show all findings" }));
    expect(params(router).has("severity")).toBe(false);
    expect(params(router).has("scope")).toBe(false);
    expect(params(router).has("q")).toBe(false);
    for (const button of groupButtons()) {
      await user.click(button);
    }
    expect(items()).toHaveLength(28);
    expect(screen.getByRole("searchbox", { name: i18next.t("report.findings.search") })).toHaveValue("");
  });

  it("a pressed severity is outlined in ink with its coloured shape; a released one is muted with a plain shape", async () => {
    await open("?severity=critical");
    expect(chip("Critical")).toHaveClass("border-fg", "bg-surface", "text-surface-fg", "hover:bg-hover");
    expect(chip("Critical").querySelector(".text-critical")).not.toBeNull();
    expect(chip("Attention")).toHaveClass("border-border", "bg-transparent", "text-muted-fg", "hover:bg-hover", "hover:text-fg");
    expect(chip("Attention").querySelector(".text-attention")).toBeNull();
    expect(chip("Critical").querySelector(".font-mono")).not.toHaveClass("italic");
  });

  it("unknown pool and cluster scopes", async () => {
    await open("?finding=20");
    expect(within(detail()).getByRole("heading", { name: "Undetermined" })).toBeInTheDocument();
    expect(within(detail()).queryByText("unknown")).toBeNull();
    expect(items()).toHaveLength(28);
    const poolItem = items().find((button) => button.textContent.includes(sentenceOf(20))) as HTMLElement;
    expect(poolItem).toHaveTextContent("Undetermined");
    expect(within(poolItem).getByText("Undetermined")).not.toHaveClass("font-mono");
    expect(within(detail()).getByRole("heading", { name: "Undetermined" })).not.toHaveClass("font-mono");
    const clusterItem = items().find((button) => button.textContent.includes(sentenceOf(1))) as HTMLElement;
    expect(clusterItem).toHaveTextContent("Cluster");
    expect(clusterItem.querySelector(".font-mono")).toBeNull();
    const user = userEvent.setup();
    await user.click(clusterItem);
    expect(within(detail()).getByText("Cluster")).toBeInTheDocument();
    expect(within(detail()).queryByRole("heading", { name: "cluster" })).toBeNull();
    expect(within(detail()).queryByText("cluster")).toBeNull();
  });

  it("from a row", async () => {
    await open("?finding=19");
    expect(chips().map((c) => c.getAttribute("aria-pressed"))).toEqual(["true", "true", "true"]);
    expect(within(detail()).getByText(sentenceOf(19))).toBeInTheDocument();
    expect(within(detail()).getByText(english.findings[19]?.action ?? "")).toBeInTheDocument();
  });

  it("the portuguese words", async () => {
    mock.session = signedIn({ language: "pt-BR" });
    await open("?finding=0", "pt-BR");
    expect(screen.getByRole("group", { name: "Severidade" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Escopo" })).toBeInTheDocument();
    expect(within(detail()).getByText("O que fazer")).toBeInTheDocument();
  });
});

describe("links into the findings tab", () => {
  it("a severity total of the overview", async () => {
    const user = userEvent.setup();
    const { router } = await renderApp(`/reports/${id}/overview`, "en");
    const totals = await screen.findByRole("group", { name: "Findings by severity" });
    await user.click(within(totals).getAllByRole("link")[0] as HTMLElement);
    expect(location(router)).toBe(`/reports/${id}/findings?severity=critical`);
    await screen.findByRole("searchbox", { name: "Search findings" });
    expect(items()).toHaveLength(10);
  });

  it("a finding of a workloads row", async () => {
    const user = userEvent.setup();
    const { router } = await renderApp(`/reports/${id}/workloads?open=payments%2FDeployment%2Fpayments-web`, "en");
    const list = await screen.findByRole("list", { name: "Findings" });
    const links = within(list).getAllByRole("link");
    const link = links.find((candidate) => candidate.textContent.includes(sentenceOf(13))) as HTMLElement;
    await user.click(link);
    expect(location(router)).toBe(`/reports/${id}/findings?finding=13`);
    await screen.findByRole("searchbox", { name: "Search findings" });
    expect(within(detail()).getByText(sentenceOf(13))).toBeInTheDocument();
    expect(within(detail()).getByText(english.findings[13]?.action ?? "")).toBeInTheDocument();
  });

  it("a finding with no action opens without the action heading", async () => {
    const quiet = english.findings.map((finding, index) => (index === 2 ? { ...finding, action: "" } : finding));
    mock.views.set(`${id}:en`, { ...english, findings: quiet });
    await open("?finding=2");
    expect(within(detail()).getByText("Observed")).toBeInTheDocument();
    expect(within(detail()).queryByText("What to do")).not.toBeInTheDocument();
    cleanup();
    await open("?finding=3");
    expect(within(detail()).getByText("What to do")).toBeInTheDocument();
  });
});

describe("findings type scale", () => {
  it("groups at 16px, detail title at 18px, section labels as labels, observed sentence at 16px, list at 14px", async () => {
    await open("?finding=0");
    const group = screen.getAllByRole("heading", { level: 2 }).find((h) => /^Critical/.test(h.textContent));
    expect(group).toHaveClass("text-base", "font-semibold");
    const title = screen.getByRole("heading", { level: 3 });
    expect(title).toHaveClass("text-lg", "font-semibold");
    for (const name of ["Observed", "What to do"]) {
      expect(screen.getByRole("heading", { level: 4, name })).toHaveClass("label");
    }
    expect(screen.getByText(sentenceOf(0), { selector: "p" })).toHaveClass("text-base");
    expect(title.closest("article")).toHaveClass("rounded-lg", "p-5", "gap-5");
    expect(items()[0]).toHaveClass("text-sm");
  });

  it("the detail draws the proof of its finding in the colour of its severity", async () => {
    await open("?finding=0");
    const bar = detail().querySelector("svg[viewBox='0 0 1000 16']") as SVGElement;
    expect(bar.querySelector("[data-fill]")).toHaveClass("fill-critical");
    expect(bar.querySelector("[data-fill]")?.getAttribute("width")).toBe("950");
    expect(bar.querySelector("[data-reference]")).toBeNull();
  });

  it("a sentence in the list keeps two lines before it is cut", async () => {
    await open();
    const entry = items()[0] as HTMLElement;
    expect(within(entry).getByText(sentenceOf(0))).toHaveClass("line-clamp-2");
  });

  it("the chart stands above the search and a segment filters the list", async () => {
    const view = await open();
    const first = english.verdict.scopes[0] as View["verdict"]["scopes"][number];
    const heading = screen.getByRole("heading", { level: 2, name: /a segment filters the list below/ });
    const search = screen.getByRole("searchbox", { name: i18next.t("report.findings.search") });
    expect(heading.compareDocumentPosition(search) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getByText("findings")).toBeInTheDocument();
    const rows = within(heading.parentElement as HTMLElement).getAllByRole("listitem");
    const link = within(rows[0] as HTMLElement).getByRole("link", { name: new RegExp(`: critical, ${String(first.critical)}$`) });
    expect(link.getAttribute("href")).toMatch(new RegExp(`\\?scope=${first.scope_type}&severity=critical$`));
    await userEvent.click(link);
    expect(params(view.router).get("scope")).toBe(first.scope_type);
    expect(params(view.router).get("severity")).toBe("critical");
    expect(params(view.router).get("finding")).toBeNull();
  });
});
