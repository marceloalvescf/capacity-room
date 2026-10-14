const margin = 8;

// Smooth scrolling unless the reader asks for reduced motion.
export function scrollBehavior(): ScrollBehavior {
  return window.matchMedia("(prefers-reduced-motion: no-preference)").matches ? "smooth" : "auto";
}

// Scrolls the window so that a row and its opened row sit between the sticky header and the bottom edge, keeping the
// row's own top in view when the pair is taller than the window.
export function revealRows(row: HTMLElement, opened: HTMLElement, head: number): void {
  const top = row.getBoundingClientRect().top;
  const bottom = opened.getBoundingClientRect().bottom;
  let by = 0;
  if (bottom > window.innerHeight - margin) {
    by = Math.min(bottom - window.innerHeight + margin, top - head - margin);
  } else if (top < head + margin) {
    by = top - head - margin;
  }
  if (by === 0) {
    return;
  }
  window.scrollBy({ top: by, behavior: scrollBehavior() });
}
