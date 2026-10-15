// Parts the items of a line by a gap. The middle dot stays for screen readers and in the text of the line, so a line
// still reads as its items; the space after the gap is where the line may break, so a wrapped line never starts with
// the gap.
export function Separator() {
  return (
    <>
      <span aria-hidden="true" className="me-4" />
      <span className="sr-only">{" ·"}</span>{" "}
    </>
  );
}
