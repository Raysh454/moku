import type { TextareaHTMLAttributes } from "react";

/** Styled multi-line input used inside `Field`. */
export function Textarea({ className = "", ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      className={`w-full resize-y rounded-lg border border-border bg-bg px-3 py-2 font-mono text-sm text-primary placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-accent/40 ${className}`}
      {...rest}
    />
  );
}
