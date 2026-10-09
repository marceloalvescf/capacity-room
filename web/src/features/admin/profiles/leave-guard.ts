import { createElement, useEffect, useRef, type ReactElement } from "react";
import { useTranslation } from "react-i18next";
import { useBlocker } from "react-router";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

// The question that precedes a loss of the changes of the form.
export function LeaveDialog({ open, onLeave, onStay }: { open: boolean; onLeave: () => void; onStay: () => void }) {
  const { t } = useTranslation();
  return createElement(ConfirmDialog, {
    open,
    title: t("admin.profile.leaveTitle"),
    body: t("admin.profile.leaveBody"),
    confirm: t("admin.profile.leave"),
    cancel: t("admin.profile.stay"),
    onConfirm: onLeave,
    onOpenChange: (next: boolean) => {
      if (!next) {
        onStay();
      }
    },
  });
}

// Asks before a navigation inside the app, and warns the browser before it unloads the page, while the form has
// changes. `release` lets the navigations of the form itself through, and so does a navigation to sign-in: the
// session has ended or the reader signed out.
export function useLeaveGuard(dirty: boolean): { release: () => void; dialog: ReactElement } {
  const releasing = useRef(false);
  const blocker = useBlocker(({ currentLocation, nextLocation }) => {
    if (nextLocation.pathname === "/login") {
      releasing.current = true;
      return false;
    }
    return dirty && !releasing.current && currentLocation.pathname !== nextLocation.pathname;
  });

  useEffect(() => {
    if (!dirty) {
      return;
    }
    function warn(event: BeforeUnloadEvent) {
      if (!releasing.current) {
        event.preventDefault();
      }
    }
    window.addEventListener("beforeunload", warn);
    return () => {
      window.removeEventListener("beforeunload", warn);
    };
  }, [dirty]);

  const dialog = createElement(LeaveDialog, {
    open: blocker.state === "blocked",
    onLeave: () => {
      blocker.proceed?.();
    },
    onStay: () => {
      blocker.reset?.();
    },
  });
  return {
    release: () => {
      releasing.current = true;
    },
    dialog,
  };
}
