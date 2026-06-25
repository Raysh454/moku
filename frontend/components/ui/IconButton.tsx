import type { ButtonHTMLAttributes } from "react";
import type { IconComponent } from "./icons";

/** Square icon button. Replaces the hand-inlined `<button><svg/></button>`
 * patterns in the topbar, sidebar, and explorer. `label` is required for a11y. */
type IconButtonTone = "ghost" | "accent" | "danger" | "success";
type IconButtonSize = "sm" | "md";

const TONE_CLASSES: Record<IconButtonTone, string> = {
  ghost: "text-helper hover:text-primary hover:bg-white/5",
  accent: "bg-accent text-on-accent hover:brightness-110",
  danger: "text-danger hover:bg-danger/10",
  success: "text-success hover:bg-success/10",
};

const SIZE_CLASSES: Record<IconButtonSize, string> = { sm: "h-7 w-7", md: "h-9 w-9" };
const ICON_CLASSES: Record<IconButtonSize, string> = { sm: "h-4 w-4", md: "h-5 w-5" };

interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> {
  icon: IconComponent;
  label: string;
  tone?: IconButtonTone;
  size?: IconButtonSize;
  active?: boolean;
}

export function IconButton({
  icon: Icon,
  label,
  tone = "ghost",
  size = "md",
  active = false,
  type = "button",
  title,
  className = "",
  ...rest
}: IconButtonProps) {
  return (
    <button
      type={type}
      aria-label={label}
      title={title ?? label}
      className={`inline-flex items-center justify-center rounded-lg transition-all active:scale-95 disabled:pointer-events-none disabled:opacity-40 ${SIZE_CLASSES[size]} ${TONE_CLASSES[tone]} ${active ? "bg-white/10 text-primary" : ""} ${className}`}
      {...rest}
    >
      <Icon className={ICON_CLASSES[size]} />
    </button>
  );
}
