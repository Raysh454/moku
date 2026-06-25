import { useEffect, type ReactNode } from "react";
import { IconButton } from "./IconButton";
import { X } from "./icons";

/** Centered modal dialog. Dedupes the three near-identical modal shells that
 * were inlined in the sidebar, explorer, and filter settings. Closes on
 * Escape and backdrop click. */
type ModalSize = "sm" | "md" | "lg";

const SIZE_CLASSES: Record<ModalSize, string> = {
  sm: "max-w-md",
  md: "max-w-2xl",
  lg: "max-w-4xl",
};

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  subtitle?: ReactNode;
  footer?: ReactNode;
  size?: ModalSize;
  children: ReactNode;
}

export function Modal({ open, onClose, title, subtitle, footer, size = "md", children }: ModalProps) {
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-[120] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
      onMouseDown={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        className={`w-full ${SIZE_CLASSES[size]} overflow-hidden rounded-xl border border-border bg-card`}
        onMouseDown={(event) => event.stopPropagation()}
      >
        {title || subtitle ? (
          <div className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
            <div className="min-w-0">
              {title ? <h2 className="text-sm font-semibold text-primary">{title}</h2> : null}
              {subtitle ? <p className="mt-0.5 text-xs text-helper">{subtitle}</p> : null}
            </div>
            <IconButton icon={X} label="Close" size="sm" onClick={onClose} />
          </div>
        ) : null}
        <div className="p-5">{children}</div>
        {footer ? <div className="flex justify-end gap-2 border-t border-border px-5 py-4">{footer}</div> : null}
      </div>
    </div>
  );
}
