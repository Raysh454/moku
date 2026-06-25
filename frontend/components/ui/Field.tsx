import type { ReactNode } from "react";

/** Label + control + hint/error row used by every form in the app. */
interface FieldProps {
  label: string;
  htmlFor?: string;
  hint?: ReactNode;
  error?: ReactNode;
  className?: string;
  children: ReactNode;
}

export function Field({ label, htmlFor, hint, error, className = "", children }: FieldProps) {
  return (
    <label htmlFor={htmlFor} className={`block ${className}`}>
      <span className="mb-1 block text-xs font-medium text-helper">{label}</span>
      {children}
      {hint && !error ? <span className="mt-1 block text-[11px] text-muted">{hint}</span> : null}
      {error ? <span className="mt-1 block text-[11px] text-danger">{error}</span> : null}
    </label>
  );
}
