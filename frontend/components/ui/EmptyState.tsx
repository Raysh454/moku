import type { ReactNode } from "react";
import type { IconComponent } from "./icons";

/** Centered empty/placeholder state. Replaces the giant opacity-30 inline-SVG
 * blocks with their `tracking-[0.4em]` shouting captions. */
interface EmptyStateProps {
  icon?: IconComponent;
  title: string;
  hint?: ReactNode;
  action?: ReactNode;
  className?: string;
}

export function EmptyState({ icon: Icon, title, hint, action, className = "" }: EmptyStateProps) {
  return (
    <div className={`flex flex-col items-center justify-center px-6 py-16 text-center ${className}`}>
      {Icon ? <Icon className="mb-4 h-10 w-10 text-helper/40" /> : null}
      <p className="text-sm font-medium text-helper">{title}</p>
      {hint ? <p className="mt-1.5 max-w-sm text-xs text-muted">{hint}</p> : null}
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}
