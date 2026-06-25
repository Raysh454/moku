import type { ButtonHTMLAttributes } from "react";
import type { IconComponent } from "./icons";

/** Text button with consistent variants. Replaces the dozens of ad-hoc
 * `bg-accent text-bg rounded px-3 py-2 ...` button strings. */
type ButtonVariant = "primary" | "secondary" | "danger" | "ghost" | "success";
type ButtonSize = "sm" | "md";

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: "bg-accent text-bg hover:brightness-110",
  success: "bg-success text-black hover:brightness-110",
  danger: "border border-danger/30 bg-danger/10 text-danger hover:bg-danger/20",
  secondary: "border border-border bg-bg text-primary hover:border-slate-500",
  ghost: "text-helper hover:bg-white/5 hover:text-primary",
};

const SIZE_CLASSES: Record<ButtonSize, string> = { sm: "h-8 px-3 text-xs", md: "h-10 px-4 text-sm" };

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: IconComponent;
}

export function Button({
  variant = "primary",
  size = "md",
  icon: Icon,
  type = "button",
  className = "",
  children,
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={`inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-all active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50 ${VARIANT_CLASSES[variant]} ${SIZE_CLASSES[size]} ${className}`}
      {...rest}
    >
      {Icon ? <Icon className="h-4 w-4" /> : null}
      {children}
    </button>
  );
}
