import * as React from "react";

/* Replaces `react-remove-scroll-bar`; vite.config.ts aliases both the package
   root and its `/constants` subpath to this module. The package measures the
   scrollbar gap and injects a `<style>` element into `<head>`, which a
   `style-src 'self'` policy refuses. This module sets and clears the
   body attribute that locks scrolling, and `src/theme/theme.css` holds the
   static rules for that attribute. It does not compensate for the scrollbar gap. */

export const lockAttribute = "data-scroll-locked";

function getCurrentUseCounter(): number {
  const counter = parseInt(document.body.getAttribute(lockAttribute) ?? "0", 10);
  return Number.isFinite(counter) ? counter : 0;
}

export function useLockAttribute(): void {
  React.useEffect(() => {
    document.body.setAttribute(lockAttribute, (getCurrentUseCounter() + 1).toString());
    return () => {
      const newCounter = getCurrentUseCounter() - 1;
      if (newCounter <= 0) {
        document.body.removeAttribute(lockAttribute);
      } else {
        document.body.setAttribute(lockAttribute, newCounter.toString());
      }
    };
  }, []);
}

export const RemoveScrollBar: React.FC = () => {
  useLockAttribute();
  return null;
};

export const zeroRightClassName = "right-scroll-bar-position";
export const fullWidthClassName = "width-before-scroll-bar";
