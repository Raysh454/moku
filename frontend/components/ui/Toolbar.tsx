import type { ReactNode } from "react";

/** Horizontal control row with consistent spacing (compare bar, action bars). */
interface ToolbarProps {
  children: ReactNode;
  className?: string;
}

export function Toolbar({ children, className = "" }: ToolbarProps) {
  return <div className={`flex items-center gap-2 ${className}`}>{children}</div>;
}
