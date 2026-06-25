import { useMemo } from "react";
import type { Snapshot } from "../../types/project";
import { getSnapshotContentInfo } from "../../lib/contentView";

type Props = {
  snapshot: Snapshot | null | undefined;
};

type SurfaceSection = {
  key: string;
  label: string;
  count: number;
  items: string[];
};

const cleanText = (value: string): string => value.replace(/\s+/g, " ").trim();

export function AttackSurfaceElementsPanel({ snapshot }: Props) {
  const content = useMemo(() => getSnapshotContentInfo(snapshot), [snapshot]);

  const sections = useMemo<SurfaceSection[]>(() => {
    const html = content.textBody;
    if (!html) return [];

    const parser = new DOMParser();
    const doc = parser.parseFromString(html, "text/html");

    const forms = Array.from(doc.querySelectorAll("form"));
    const controls = Array.from(doc.querySelectorAll("input, textarea, select"));
    const buttons = Array.from(doc.querySelectorAll("button, input[type='submit'], input[type='button']"));
    const links = Array.from(doc.querySelectorAll("a[href]"));
    const scripts = Array.from(doc.querySelectorAll("script[src]"));
    const iframes = Array.from(doc.querySelectorAll("iframe"));

    const toTopItems = <T,>(elements: T[], render: (item: T) => string, limit = 6) =>
      elements
        .slice(0, limit)
        .map((item) => cleanText(render(item)))
        .filter(Boolean);

    const formItems = toTopItems(forms, (form) => {
      const method = (form.getAttribute("method") || "GET").toUpperCase();
      const action = form.getAttribute("action") || "(same page)";
      const formControls = form.querySelectorAll("input, textarea, select, button").length;
      return `${method} ${action} • controls: ${formControls}`;
    });

    const controlItems = toTopItems(controls, (control) => {
      const tag = control.tagName.toLowerCase();
      const type = control.getAttribute("type") || tag;
      const name = control.getAttribute("name") || control.getAttribute("id") || "(unnamed)";
      return `${type} • ${name}`;
    });

    const buttonItems = toTopItems(buttons, (button) => {
      const text = cleanText(button.textContent || "");
      const type = button.getAttribute("type") || "button";
      return `${type} • ${text || "(no label)"}`;
    });

    const linkItems = toTopItems(links, (link) => {
      const href = link.getAttribute("href") || "";
      const text = cleanText(link.textContent || "");
      return `${href} • ${text || "(no text)"}`;
    });

    const scriptItems = toTopItems(scripts, (script) => script.getAttribute("src") || "");
    const iframeItems = toTopItems(iframes, (iframe) => iframe.getAttribute("src") || "(inline/empty src)");

    const sensitiveInputCount = controls.filter((control) => {
      const type = (control.getAttribute("type") || "").toLowerCase();
      return type === "password" || type === "file" || type === "hidden";
    }).length;

    return [
      { key: "forms", label: "Forms", count: forms.length, items: formItems },
      { key: "controls", label: "Inputs & controls", count: controls.length, items: controlItems },
      { key: "buttons", label: "Buttons", count: buttons.length, items: buttonItems },
      { key: "links", label: "Links", count: links.length, items: linkItems },
      { key: "scripts", label: "External scripts", count: scripts.length, items: scriptItems },
      { key: "iframes", label: "Iframes", count: iframes.length, items: iframeItems },
      {
        key: "sensitive-inputs",
        label: "Sensitive inputs",
        count: sensitiveInputCount,
        items: [],
      },
    ];
  }, [content.textBody]);

  if (!snapshot?.body) {
    return <p className="text-xs text-slate-500">No head snapshot HTML loaded for this endpoint/version.</p>;
  }

  if (content.viewKind !== "html" && content.viewKind !== "directory") {
    return (
      <p className="text-xs text-slate-500">
        Attack-surface element extraction is available for HTML pages. Current content type:{" "}
        <span className="text-slate-300">{content.contentType}</span>.
      </p>
    );
  }

  if (sections.every((section) => section.count === 0)) {
    return <p className="text-xs text-slate-500">No attack-surface elements detected in the head page HTML.</p>;
  }

  return (
    <div className="grid grid-cols-1 gap-x-8 gap-y-5 lg:grid-cols-2">
      {sections
        .filter((section) => section.count > 0)
        .map((section) => (
          <article key={section.key} className="space-y-1.5">
            <div className="flex items-center justify-between gap-2 border-b border-border/50 pb-1">
              <span className="text-xs font-semibold text-helper">{section.label}</span>
              <span className="text-[11px] tabular-nums text-muted">{section.count}</span>
            </div>
            {section.items.length > 0 && (
              <ul className="space-y-1 text-xs text-slate-300">
                {section.items.map((item, index) => (
                  <li key={`${section.key}-${index}`} className="truncate" title={item}>
                    {item}
                  </li>
                ))}
              </ul>
            )}
          </article>
        ))}
    </div>
  );
}
