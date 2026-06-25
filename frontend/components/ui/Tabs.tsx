import { useRef, type KeyboardEvent, type ReactNode } from "react";
import type { IconComponent } from "./icons";

/**
 * Controlled segmented tab control. One implementation backs every tab strip
 * in the app (workspace views, diff split/unified, settings tabs), replacing
 * the bespoke uppercase button rows. Accessible: `role="tablist"` with
 * arrow-key navigation.
 */
export interface TabItem {
  id: string;
  label: string;
  icon?: IconComponent;
  badge?: ReactNode;
}

interface TabsProps {
  items: TabItem[];
  value: string;
  onChange: (id: string) => void;
  ariaLabel: string;
  size?: "sm" | "md";
  className?: string;
}

const SIZE_CLASSES = {
  sm: "px-3 py-1 text-xs",
  md: "px-4 py-1.5 text-sm",
} as const;

export function Tabs({ items, value, onChange, ariaLabel, size = "md", className = "" }: TabsProps) {
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const focusTab = (index: number) => {
    const wrapped = (index + items.length) % items.length;
    onChange(items[wrapped].id);
    tabRefs.current[wrapped]?.focus();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    if (event.key === "ArrowRight" || event.key === "ArrowDown") {
      event.preventDefault();
      focusTab(index + 1);
    } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
      event.preventDefault();
      focusTab(index - 1);
    }
  };

  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      className={`inline-flex items-center gap-1 rounded-lg border border-border bg-bg/60 p-1 ${className}`}
    >
      {items.map((item, index) => {
        const isActive = item.id === value;
        const Icon = item.icon;
        return (
          <button
            key={item.id}
            ref={(node) => {
              tabRefs.current[index] = node;
            }}
            role="tab"
            type="button"
            aria-selected={isActive}
            tabIndex={isActive ? 0 : -1}
            onClick={() => onChange(item.id)}
            onKeyDown={(event) => onKeyDown(event, index)}
            className={`inline-flex items-center gap-2 rounded-md font-medium transition-colors ${SIZE_CLASSES[size]} ${
              isActive ? "bg-accent text-white shadow-sm" : "text-helper hover:text-primary hover:bg-white/5"
            }`}
          >
            {Icon ? <Icon className="h-4 w-4" /> : null}
            {item.label}
            {item.badge != null ? <span className="ml-0.5 text-[10px] opacity-80">{item.badge}</span> : null}
          </button>
        );
      })}
    </div>
  );
}
