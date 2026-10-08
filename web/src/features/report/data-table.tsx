import { CaretDownIcon, CaretRightIcon, CaretUpIcon, MagnifyingGlassIcon } from "@phosphor-icons/react";
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
import { Fragment, useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import { IconButton } from "@/components/ui/icon-button";
import { Field } from "@/components/ui/field";
import { cn } from "@/lib/cn";

export interface ReportColumn<Row> {
  id: string;
  header: string;
  cell: (row: Row) => ReactNode;
  // A column without a sort value does not sort; a null sorts after every value.
  sortValue?: (row: Row) => number | string | null;
  align?: "start" | "end";
}

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
  searchLabel,
  rows,
  rowKey,
  rowName,
  columns,
  searchText,
  renderOpened,
  defaultOrder = "status",
  status,
}: {
  label: string;
  searchLabel: string;
  rows: readonly Row[];
  rowKey: (row: Row) => string;
  rowName: (row: Row) => string;
  columns: ReportColumn<Row>[];
  searchText: (row: Row) => string;
  renderOpened: (row: Row) => ReactNode;
  defaultOrder?: "status" | "view";
  status?: (row: Row) => string;
}) {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const query = params.get("q") ?? "";
  const sortable = useMemo(
    () => new Set(columns.filter((column) => column.sortValue !== undefined).map((column) => column.id)),
    [columns],
  );
  const sorting = readSorting(params.get("sort"), sortable);
  const openKey = params.get("open");

  const data = useMemo(() => {
    const needle = query.toLowerCase();
    const kept = rows.filter((row) => searchText(row).toLowerCase().includes(needle));
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

  function write(change: (next: URLSearchParams) => void) {
    const next = new URLSearchParams(params);
    change(next);
    setParams(next, { replace: true });
  }

  function onSortingChange(updater: Updater<SortingState>) {
    const [entry] = typeof updater === "function" ? updater(sorting) : updater;
    write((next) => {
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
    onSortingChange,
    getRowId: rowKey,
  });

  const search = (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div className="relative">
        <Field
          label={searchLabel}
          type="search"
          value={query}
          className="w-72 max-w-full pl-9"
          onChange={(event) => {
            write((next) => {
              if (event.target.value === "") {
                next.delete("q");
              } else {
                next.set("q", event.target.value);
              }
            });
          }}
        />
        <MagnifyingGlassIcon aria-hidden="true" className="pointer-events-none absolute bottom-2.5 left-3 size-4 text-muted-fg" />
      </div>
      <p className="text-sm text-muted-fg">{t("report.table.rows", { count: data.length })}</p>
    </div>
  );

  if (rows.length === 0) {
    return <p className="text-sm text-muted-fg">{t("report.table.empty")}</p>;
  }

  const byId = new Map(columns.map((column) => [column.id, column]));
  const cellClass = (column: ReportColumn<Row> | undefined) =>
    cn("px-3 py-2 align-top", column?.align === "end" ? "text-right" : "text-left");

  return (
    <div className="flex flex-col gap-3">
      {search}
      {data.length === 0 ? (
        <p className="text-sm text-muted-fg">{t("report.table.noMatch")}</p>
      ) : (
        <div className="rounded-md border border-border bg-surface text-surface-fg">
          <table aria-label={label} className="w-full border-separate border-spacing-0 text-sm">
            <thead className="sticky top-0 z-10 bg-surface text-muted-fg [&>tr>th]:border-b [&>tr>th]:border-border">
              {table.getHeaderGroups().map((group) => (
                <tr key={group.id}>
                  <td className="w-12 px-3 py-2" />
                  {group.headers.map((header) => {
                    const column = byId.get(header.column.id);
                    const direction = header.column.getIsSorted();
                    return (
                      <th
                        key={header.id}
                        aria-sort={direction === false ? undefined : direction === "asc" ? "ascending" : "descending"}
                        className={cn("px-3 py-2 font-medium", column?.align === "end" ? "text-right" : "text-left")}
                      >
                        {header.column.getCanSort() ? (
                          <button
                            type="button"
                            onClick={header.column.getToggleSortingHandler()}
                            className="inline-flex items-center gap-1 font-medium focus-visible:outline-2 focus-visible:outline-accent"
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
            <tbody className="[&>tr>td]:border-b [&>tr>td]:border-border">
              {table.getRowModel().rows.map((row) => {
                const opened = row.id === openKey;
                const name = rowName(row.original);
                return (
                  <Fragment key={row.id}>
                    <tr className={cn(opened && "bg-info-surface")}>
                      <td className="px-3 py-1 align-top">
                        <IconButton
                          label={t(opened ? "report.table.close" : "report.table.open", { name })}
                          variant="secondary"
                          aria-expanded={opened}
                          className="border-transparent bg-transparent"
                          onClick={() => {
                            write((next) => {
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
                      {row.getAllCells().map((cell) => {
                        const column = byId.get(cell.column.id);
                        return (
                          <td key={cell.id} className={cellClass(column)}>
                            {column?.cell(row.original)}
                          </td>
                        );
                      })}
                    </tr>
                    {opened ? (
                      <tr className="bg-bg">
                        <td colSpan={columns.length + 1} className="px-6 py-4">
                          {renderOpened(row.original)}
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
