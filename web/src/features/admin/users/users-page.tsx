import { useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useAnnounce } from "@/app/live-region";
import { useScreenTitle } from "@/app/title";
import { DataTable, type Column } from "@/components/data-table";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { useAddress } from "@/lib/address";
import { usePrincipal } from "@/session/session";
import { useAdminUsers, type User } from "./api";
import { NewUserDialog } from "./new-user-dialog";
import { UserRow } from "./user-row";

const rowKey = (user: User) => user.id;
const rowName = (user: User) => user.username;
const renderOpened = (user: User) => <UserRow user={user} />;

export function UsersPage() {
  const { t } = useTranslation();
  const users = useAdminUsers();
  const principal = usePrincipal();
  const address = useAddress();
  const announce = useAnnounce();
  const [creating, setCreating] = useState(false);
  const opener = useRef<HTMLButtonElement>(null);
  useScreenTitle(t("admin.users.title"));
  const ownId = principal?.id;

  const columns = useMemo<Column<User>[]>(
    () => [
      {
        id: "user",
        header: t("admin.users.column.user"),
        cell: (user) => (
          <>
            <span className="font-mono">{user.username}</span>
            {user.id === ownId ? <span className="text-muted-fg">{` ${t("admin.users.you")}`}</span> : null}
          </>
        ),
      },
      { id: "role", header: t("admin.users.column.role"), cell: (user) => t(`admin.users.role.${user.role}`) },
      {
        id: "state",
        header: t("admin.users.column.state"),
        cell: (user) =>
          user.active ? (
            t("admin.state.active")
          ) : (
            <span className="text-muted-fg">{t("admin.state.inactive")}</span>
          ),
      },
    ],
    [t, ownId],
  );

  let content = null;
  if (users.data !== undefined) {
    content = (
      <DataTable
        label={t("admin.users.title")}
        rows={users.data}
        rowKey={rowKey}
        rowName={rowName}
        columns={columns}
        renderOpened={renderOpened}
        empty={t("admin.users.empty")}
        defaultOrder="view"
      />
    );
  } else if (users.isError) {
    content = <Alert tone="error">{t("admin.users.loadFailed")}</Alert>;
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">{t("admin.users.title")}</h1>
        <Button
          ref={opener}
          variant="primary"
          onClick={() => {
            setCreating(true);
          }}
        >
          {t("admin.users.new")}
        </Button>
      </div>
      {content}
      <NewUserDialog
        open={creating}
        onOpenChange={setCreating}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          opener.current?.focus();
        }}
        onCreated={(user) => {
          setCreating(false);
          announce(t("admin.users.createdNotice", { name: user.username }));
          address.write((next) => {
            next.set("open", user.id);
          });
        }}
      />
    </div>
  );
}
