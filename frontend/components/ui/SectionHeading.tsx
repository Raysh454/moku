import type { ReactNode } from "react";
import type { IconComponent } from "./icons";

/**
 * Sentence-case section heading. Deliberately NOT the tiny ALL-CAPS
 * wide-tracked eyebrow the old UI used everywhere — that pattern is the main
 * "slop" tell this revamp removes.
 */
interface SectionHeadingProps {
  title: string;
  count?: number | string;
  actions?: ReactNode;
  icon?: IconComponent;
  size?: "section" | "sub";
  className?: string;
}

export function SectionHeading({ title, count, actions, icon: Icon, size = "section", className = "" }: SectionHeadingProps) {
  const titleClass = size === "section" ? "text-sm font-semibold text-primary" : "text-xs font-semibold text-helper";
  return (
    <div className={`flex items-center justify-between gap-3 ${className}`}>
      <div className="flex min-w-0 items-center gap-2">
        {Icon ? <Icon className="h-4 w-4 shrink-0 text-helper" /> : null}
        <h3 className={`truncate ${titleClass}`}>{title}</h3>
        {count != null ? <span className="text-xs tabular-nums text-muted">{count}</span> : null}
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-1">{actions}</div> : null}
    </div>
  );
}
