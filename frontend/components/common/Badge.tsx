import React from "react";
import { Severity, ProjectStatus } from "../../types/project";

interface BadgeProps {
  variant?: Severity | ProjectStatus | "default";
  children: React.ReactNode;
  className?: string;
}

const colorMap: Record<string, string> = {
  info: "bg-accent/10 text-accent border-accent/20",
  low: "bg-accent/10 text-accent border-accent/20",
  medium: "bg-warning/10 text-warning border-warning/20",
  high: "bg-danger/10 text-danger border-danger/20",
  critical:
    "bg-danger text-white border-danger shadow-lg shadow-danger/20 font-semibold",

  active: "bg-success/10 text-success border-success/20",
  monitoring: "bg-warning/10 text-warning border-warning/20",
  idle: "bg-white/5 text-slate-500 border-border",

  default: "bg-white/5 text-slate-400 border-border",
};

export const Badge: React.FC<BadgeProps> = ({
  variant = "default",
  children,
  className = "",
}) => {
  const styles = colorMap[variant] || colorMap.default;
  return (
    <span
      className={`px-2 py-0.5 text-[9px] font-medium uppercase tracking-[0.1em] rounded-md border tabular-nums inline-flex items-center justify-center ${styles} ${className}`}
    >
      {children}
    </span>
  );
};
