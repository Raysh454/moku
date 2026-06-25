import type { InputHTMLAttributes } from "react";

/** Labelled checkbox row. */
interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "type"> {
  label: string;
}

export function Checkbox({ label, className = "", ...rest }: CheckboxProps) {
  return (
    <label className={`inline-flex cursor-pointer items-center gap-2 text-xs text-primary ${className}`}>
      <input type="checkbox" className="h-3.5 w-3.5 accent-accent" {...rest} />
      {label}
    </label>
  );
}
