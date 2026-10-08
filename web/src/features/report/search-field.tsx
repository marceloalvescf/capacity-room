import { MagnifyingGlassIcon } from "@phosphor-icons/react";
import { useState } from "react";
import { Field } from "@/components/ui/field";
import type { Address } from "./address";

// The text of a search field, kept apart from the address so that a keystroke never waits for the router. It follows
// the address when the address changes from outside, and writes `q` as it is typed.
export function useSearchText(address: Address): { text: string; change: (value: string) => void } {
  const query = address.params.get("q") ?? "";
  const [text, setText] = useState(query);
  const [seen, setSeen] = useState(address.outside);
  if (seen !== address.outside) {
    setSeen(address.outside);
    setText(query);
  }
  return {
    text,
    change: (value) => {
      setText(value);
      address.write((next) => {
        if (value === "") {
          next.delete("q");
        } else {
          next.set("q", value);
        }
      });
    },
  };
}

export function SearchField({ label, text, onChange }: { label: string; text: string; onChange: (value: string) => void }) {
  return (
    <div className="relative">
      <Field
        label={label}
        type="search"
        value={text}
        className="w-72 max-w-full pl-9"
        onChange={(event) => {
          onChange(event.target.value);
        }}
      />
      <MagnifyingGlassIcon aria-hidden="true" className="pointer-events-none absolute bottom-2.5 left-3 size-4 text-muted-fg" />
    </div>
  );
}
