import { CaretDownIcon, CaretRightIcon, CaretUpIcon } from "@phosphor-icons/react";
import {
  createSortedRowModel,
  rowSortingFeature,
  tableFeatures,
  useTable,
  type ColumnDef,
  type RowData,
  type SortingState,
  type Updater,
} from "@tanstack/react-table";
import { Fragment, useEffect, useLayoutEffect, useMemo, useRef, type KeyboardEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useAnnounce } from "@/app/live-region";
import { IconButton } from "@/components/ui/icon-button";
import { cn } from "@/lib/cn";
import { useAddress } from "@/lib/address";
import { SearchField, useSearchText } from "./search-field";
import { revealRows } from "./reveal";

export interface Column<Row> {
  id: string;
  header: string;
  cell: (row: Row) => ReactNode;
  // A column without a sort value does not sort; a null sorts after every value.
  sortValue?: (row: Row) => number | string | null;
  align?: "start" | "end";
  // Classes a cell of this column takes for its row, such as the tint of a value by its status.
  cellClassName?: (row: Row) => string;
}

// The layout of a set of facts inside an opened row: as many columns of at least 14rem as the row holds.
export const factGrid = "grid gap-4 grid-cols-[repeat(auto-fit,minmax(14rem,1fr))]";

const features = tableFeatures({ rowSortingFeature, sortedRowModel: createSortedRowModel() });

export const severityRank: Readonly<Record<string, number>> = { critical: 0, attention: 1, healthy: 2 };

function compare(a: unknown, b: unknown): number {
  if (typeof a === "number" && typeof b === "number") {
    return a - b;
  }
  const left = String(a);
  const right = String(b);
  return left < right ? -1 : left > right ? 1 : 0;
}

// The sorting the address holds: a known sortable column and a known direction, or none.
function readSorting(param: string | null, sortable: ReadonlySet<string>): SortingState {
  if (param === null) {
    return [];
  }
  const dot = param.lastIndexOf(".");
  const id = param.slice(0, dot);
  const direction = param.slice(dot + 1);
  if (dot < 0 || !sortable.has(id) || (direction !== "asc" && direction !== "desc")) {
    return [];
  }
  return [{ id, desc: direction === "desc" }];
}

function Sorted({ direction }: { direction: false | "asc" | "desc" }) {
  if (direction === false) {
    return null;
  }
  const Icon = direction === "asc" ? CaretUpIcon : CaretDownIcon;
  return <Icon aria-hidden="true" className="size-3" />;
}

export function DataTable<Row extends RowData>({
  label,
  rows,
  rowKey,
  rowName,
  columns,
  renderOpened,
  empty,
  search,
  defaultOrder = "status",
  status,
}: {
  label: string;
  rows: readonly Row[];
  rowKey: (row: Row) => string;
  rowName: (row: Row) => string;
  columns: Column<Row>[];
  // Without it the rows open nothing: no expander column, and `open` in the address is ignored.
  renderOpened?: (row: Row) => ReactNode;
  empty: string;
  search?: { label: string; text: (row: Row) => string };
  defaultOrder?: "status" | "view";
  status?: (row: Row) => string;
}) {
  const { t } = useTranslation();
  const address = useAddress();
  const { params } = address;
  const searchState = useSearchText(address);
  const announce = useAnnounce();
  const query = search === undefined ? "" : searchState.text;
  const searchText = search?.text;
  const sortable = useMemo(
    () => new Set(columns.filter((column) => column.sortValue !== undefined).map((column) => column.id)),
    [columns],
  );
  const sorting = readSorting(params.get("sort"), sortable);
  const openKey = renderOpened === undefined ? null : params.get("open");

  const data = useMemo(() => {
    const needle = query.toLowerCase();
    const kept = searchText === undefined ? rows : rows.filter((row) => searchText(row).toLowerCase().includes(needle));
    if (defaultOrder === "view") {
      return kept;
    }
    const rank = (row: Row) => severityRank[status?.(row) ?? ""] ?? 3;
    return kept.toSorted((a, b) => rank(a) - rank(b) || compare(rowName(a), rowName(b)));
  }, [rows, query, searchText, defaultOrder, status, rowName]);

  const defs = useMemo<ColumnDef<typeof features, Row>[]>(
    () =>
      columns.map((column) => ({
        id: column.id,
        accessorFn: (row: Row) => column.sortValue?.(row) ?? undefined,
        enableSorting: column.sortValue !== undefined,
        sortUndefined: "last",
        sortDescFirst: false,
        sortFn: (a, b, id) => compare(a.getValue(id), b.getValue(id)),
      })),
    [columns],
  );

  function onSortingChange(updater: Updater<SortingState>) {
    const [entry] = typeof updater === "function" ? updater(sorting) : updater;
    address.write((next) => {
      if (entry === undefined) {
        next.delete("sort");
      } else {
        next.set("sort", `${entry.id}.${entry.desc ? "desc" : "asc"}`);
      }
    });
  }

  const table = useTable({
    features,
    columns: defs,
    data,
    state: { sorting },
    enableMultiSort: false,
    onSortingChange,
    getRowId: rowKey,
  });

  const announced = useRef(query);
  useEffect(() => {
    if (announced.current !== query) {
      announced.current = query;
      announce(t("table.rows", { count: data.length }));
    }
  }, [query, data.length, announce, t]);

  const head = useRef<HTMLTableSectionElement>(null);
  const openedRow = useRef<HTMLTableRowElement>(null);
  const openedPair = useRef<HTMLTableRowElement>(null);
  const revealed = useRef<string | null>(null);
  // A row is revealed once its elements exist: one filtered out when the address named it is revealed when shown.
  useLayoutEffect(() => {
    if (openKey === null) {
      revealed.current = null;
    } else if (openKey !== revealed.current && openedRow.current && openedPair.current && head.current) {
      revealRows(openedRow.current, openedPair.current, head.current.getBoundingClientRect().height);
      revealed.current = openKey;
    }
  }, [openKey, data]);

  const body = useRef<HTMLTableSectionElement>(null);
  // Active only while an opener has focus. The default of all four keys is prevented there, the first and last row
  // included, so that an arrow does not scroll the page from a row that has no neighbour.
  function moveFocus(event: KeyboardEvent<HTMLElement>) {
    const step = event.key === "j" || event.key === "ArrowDown" ? 1 : event.key === "k" || event.key === "ArrowUp" ? -1 : 0;
    if (step === 0 || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey || event.nativeEvent.isComposing) {
      return;
    }
    event.preventDefault();
    // The openers of this table's own rows, so that a table inside an opened row never joins the walk.
    const openers = Array.from(body.current?.rows ?? [])
      .map((row) => row.cells[0]?.querySelector<HTMLElement>(":scope > [data-opener]"))
      .filter((opener) => opener !== null && opener !== undefined);
    const target = openers[openers.indexOf(event.currentTarget) + step];
    const row = target?.closest("tr");
    if (target === undefined || row === null || row === undefined || head.current === null) {
      return;
    }
    target.focus({ preventScroll: true });
    revealRows(row, row, head.current.getBoundingClientRect().height);
  }

  const searchBar =
    search === undefined ? null : (
      <div className="flex flex-wrap items-end justify-between gap-3">
        <SearchField label={search.label} text={searchState.text} onChange={searchState.change} />
        <p className="text-sm text-muted-fg">{t("table.rows", { count: data.length })}</p>
      </div>
    );

  if (rows.length === 0) {
    return <p className="text-sm text-muted-fg">{empty}</p>;
  }

  const byId = new Map(columns.map((column) => [column.id, column]));
  const cellClass = (column: Column<Row> | undefined, row: Row) =>
    cn(
      "px-3 py-2 align-top",
      column?.align === "end" ? "text-right" : "text-left",
      column?.cellClassName?.(row),
    );

  return (
    <div className="flex flex-col gap-3">
      {searchBar}
      {data.length === 0 ? (
        <p className="text-sm text-muted-fg">{t("table.noMatch")}</p>
      ) : (
        <div className="rounded-lg border border-border bg-surface text-surface-fg">
          <table aria-label={label} className="w-full border-separate border-spacing-0 text-sm">
            <thead ref={head} className="sticky top-0 z-10 [&>tr>th]:border-b [&>tr>th]:border-border [&>tr>th]:bg-surface [&>tr>th:first-child]:rounded-tl-lg [&>tr>th:last-child]:rounded-tr-lg">
              {table.getHeaderGroups().map((group) => (
                <tr key={group.id}>
                  {renderOpened === undefined ? null : (
                    <th scope="col" className="w-12 px-3 py-2">
                      <span className="sr-only">{t("table.details")}</span>
                    </th>
                  )}
                  {group.headers.map((header) => {
                    const column = byId.get(header.column.id);
                    const direction = header.column.getIsSorted();
                    return (
                      <th
                        key={header.id}
                        aria-sort={direction === false ? undefined : direction === "asc" ? "ascending" : "descending"}
                        className={cn("label px-3 py-2", column?.align === "end" ? "text-right" : "text-left")}
                      >
                        {header.column.getCanSort() ? (
                          <button
                            type="button"
                            onClick={header.column.getToggleSortingHandler()}
                            className="inline-flex items-center gap-1 focus-visible:outline-2 focus-visible:outline-accent"
                          >
                            {column?.header}
                            <Sorted direction={direction} />
                          </button>
                        ) : (
                          column?.header
                        )}
                      </th>
                    );
                  })}
                </tr>
              ))}
            </thead>
            <tbody ref={body} className="[&>tr>td]:border-b [&>tr>td]:border-border">
              {table.getRowModel().rows.map((row) => {
                const opened = row.id === openKey;
                const name = rowName(row.original);
                return (
                  <Fragment key={row.id}>
                    <tr ref={opened ? openedRow : undefined} className={cn(opened && "bg-info-surface")}>
                      {renderOpened === undefined ? null : (
                        <td className="px-3 py-1 align-top">
                          <IconButton
                            label={t(opened ? "table.close" : "table.open", { name })}
                            variant="secondary"
                            aria-expanded={opened}
                            aria-keyshortcuts="j k ArrowDown ArrowUp"
                            data-opener=""
                            onKeyDown={moveFocus}
                            className="border-transparent bg-transparent"
                            onClick={() => {
                              address.write((next) => {
                                if (opened) {
                                  next.delete("open");
                                } else {
                                  next.set("open", row.id);
                                }
                              });
                            }}
                          >
                            {opened ? (
                              <CaretDownIcon aria-hidden="true" className="size-4" />
                            ) : (
                              <CaretRightIcon aria-hidden="true" className="size-4" />
                            )}
                          </IconButton>
                        </td>
                      )}
                      {row.getAllCells().map((cell) => {
                        const column = byId.get(cell.column.id);
                        return (
                          <td key={cell.id} className={cellClass(column, row.original)}>
                            {column?.cell(row.original)}
                          </td>
                        );
                      })}
                    </tr>
                    {opened && renderOpened !== undefined ? (
                      <tr ref={openedPair} className="bg-bg">
                        <td colSpan={columns.length + 1} className="px-6 py-4">
                          {/* An opened row spans the table, so only a value longer than the table breaks: an address, a list. */}
                          <div className="min-w-0 [overflow-wrap:anywhere]">{renderOpened(row.original)}</div>
                        </td>
                      </tr>
                    ) : null}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
