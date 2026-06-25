import { useEffect, useRef, useState } from "react";
import { useTheme } from "../../context/ThemeContext";
import { IconButton } from "./IconButton";
import { Palette, Check } from "./icons";

/** Palette button + dropdown for switching the app theme. */
export function ThemeMenu() {
  const { theme, themes, setThemeId } = useTheme();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <IconButton icon={Palette} label="Theme" active={open} onClick={() => setOpen((value) => !value)} />
      {open ? (
        <div className="absolute right-0 top-11 z-50 w-44 rounded-lg border border-border bg-card p-1">
          <p className="px-2.5 py-1 text-[11px] text-muted">Theme</p>
          {themes.map((option) => (
            <button
              key={option.id}
              onClick={() => {
                setThemeId(option.id);
                setOpen(false);
              }}
              className="flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-sm text-helper transition-colors hover:bg-white/5 hover:text-primary"
            >
              <span className="flex items-center gap-2">
                <span className="h-3 w-3 rounded-full" style={{ backgroundColor: option.colors.accent }} />
                {option.name}
              </span>
              {option.id === theme.id ? <Check className="h-4 w-4 text-accent" /> : null}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
