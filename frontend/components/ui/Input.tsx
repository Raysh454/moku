import type { InputHTMLAttributes } from "react";

/** Styled text/number input used inside `Field`. */
export function Input({ className = "", ...rest }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={`w-full rounded-lg border border-border bg-bg px-3 py-2 text-sm text-primary placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-accent/40 ${className}`}
      {...rest}
    />
  );
}
