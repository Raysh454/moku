import type { ReactNode } from "react";
import type { IconComponent } from "./icons";

/**
 * Small status/score chip. Replaces the repeated `statusToneClass` ternaries
 * and ad-hoc `changeKindBadge` spans scattered through the old UI.
 */
export type StatusTone = "success" | "danger" | "warning" | "accent" | "neutral";

const TONE_CLASSES: Record<StatusTone, string> = {
  success: "text-success border-success/40 bg-success/10",
  danger: "text-danger border-danger/40 bg-danger/10",
  warning: "text-warning border-warning/40 bg-warning/10",
  accent: "text-accent border-accent/40 bg-accent/10",
  neutral: "text-helper border-border bg-bg/50",
};

const HTTP_OK_FLOOR = 200;
const HTTP_REDIRECT_FLOOR = 300;
const HTTP_ERROR_FLOOR = 400;

/** Maps an HTTP status code to a chip tone. */
export function httpStatusTone(statusCode: number): StatusTone {
  if (statusCode >= HTTP_OK_FLOOR && statusCode < HTTP_REDIRECT_FLOOR) return "success";
  if (statusCode >= HTTP_ERROR_FLOOR) return "danger";
  if (statusCode >= HTTP_REDIRECT_FLOOR) return "warning";
  return "neutral";
}

/**
 * Maps a security-score delta to a tone. A higher score means more exposure,
 * so a positive delta is a regression (danger) and a negative delta an
 * improvement (success) — matching `scoreDirection` in `lib/score.ts`.
 */
export function scoreTone(delta: number): StatusTone {
  if (delta > 0) return "danger";
  if (delta < 0) return "success";
  return "neutral";
}

interface StatusPillProps {
  tone?: StatusTone;
  icon?: IconComponent;
  title?: string;
  className?: string;
  children: ReactNode;
}

export function StatusPill({ tone = "neutral", icon: Icon, title, className = "", children }: StatusPillProps) {
  return (
    <span
      title={title}
      className={`inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-[11px] font-medium tabular-nums ${TONE_CLASSES[tone]} ${className}`}
    >
      {Icon ? <Icon className="h-3 w-3" /> : null}
      {children}
    </span>
  );
}
