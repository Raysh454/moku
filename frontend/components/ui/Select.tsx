import type { SelectHTMLAttributes } from "react";
import { ChevronDown } from "./icons";

/** Styled wrapper over a native `<select>` (kept native for a11y and to
 * preserve the lazy version-refresh `onFocus`/`onMouseDown` hooks the
 * compare pickers rely on). */
interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  sizeVariant?: "sm" | "md";
}

export function Select({ sizeVariant = "md", className = "", children, ...rest }: SelectProps) {
  const sizeClass = sizeVariant === "sm" ? "py-1 pl-2 pr-7 text-xs" : "py-2 pl-3 pr-8 text-sm";
  return (
    <div className="relative inline-flex w-full items-center">
      <select
        className={`w-full appearance-none rounded-lg border border-border bg-bg text-primary focus:outline-none focus:ring-1 focus:ring-accent/40 ${sizeClass} ${className}`}
        {...rest}
      >
        {children}
      </select>
      <ChevronDown className="pointer-events-none absolute right-2 h-4 w-4 text-helper" />
    </div>
  );
}
