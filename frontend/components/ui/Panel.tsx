import type { ElementType, ReactNode } from "react";

/**
 * The single card/surface primitive. Replaces the repeated
 * `bg-card border border-border rounded-xl p-5` soup. Use `tone="sunken"`
 * for nested regions instead of nesting another bordered card.
 */
type PanelTone = "card" | "sunken" | "plain";
type PanelPadding = "none" | "sm" | "md" | "lg";

const TONE_CLASSES: Record<PanelTone, string> = {
  card: "bg-card border border-border",
  sunken: "bg-bg/40 border border-border",
  plain: "",
};

const PADDING_CLASSES: Record<PanelPadding, string> = {
  none: "",
  sm: "p-3",
  md: "p-4",
  lg: "p-6",
};

interface PanelProps {
  as?: ElementType;
  tone?: PanelTone;
  padding?: PanelPadding;
  className?: string;
  children: ReactNode;
}

export function Panel({ as: Tag = "div", tone = "card", padding = "md", className = "", children }: PanelProps) {
  return <Tag className={`rounded-xl ${TONE_CLASSES[tone]} ${PADDING_CLASSES[padding]} ${className}`}>{children}</Tag>;
}
