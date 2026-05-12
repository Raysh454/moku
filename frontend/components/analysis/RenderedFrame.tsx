import { useCallback, useEffect, useRef, useState } from "react";
import type { AttackSurfaceChange } from "../../src/api/types";

export type HighlightedElement = {
  type: string;
  domIndex?: number;
  parentDomIndex?: number;
  domId?: string;
  domClasses?: string[];
  change?: AttackSurfaceChange;
};

export type TextChange = {
  type: "added" | "removed" | "modified";
  content: string;
  position?: number;
  length?: number;
};

type RenderedFrameProps = {
  html: string;
  title?: string;
  highlights?: HighlightedElement[];
  activeHighlight?: HighlightedElement | null;
  showHighlights?: boolean;
  textChanges?: TextChange[];
  showTextHighlights?: boolean;
  className?: string;
};

const VOID_ELEMENTS = [
  "input",
  "br",
  "hr",
  "img",
  "meta",
  "link",
  "area",
  "base",
  "col",
  "embed",
  "param",
  "source",
  "track",
  "wbr",
];

const getHighlightScript = (
  highlights: HighlightedElement[],
  activeIndex: number | null,
  showHighlights: boolean,
  showTextHighlights: boolean,
) => `
  (function() {
    document.body.classList.toggle('moku-highlights-hidden', !${showHighlights});
    document.body.classList.toggle('moku-text-hidden', !${showTextHighlights});
    document.querySelectorAll('.moku-highlight-wrapper').forEach(wrapper => {
      const original = wrapper.firstChild;
      if (original) wrapper.parentNode.replaceChild(original, wrapper);
    });
    document.querySelectorAll('.moku-highlight').forEach(el => {
      el.classList.remove('moku-highlight', 'moku-highlight-active', 'moku-highlight-added', 'moku-highlight-removed', 'moku-highlight-changed');
      el.removeAttribute('data-moku-change');
    });

    const highlights = ${JSON.stringify(highlights)};
    const activeIndex = ${activeIndex};
    const voidElements = ${JSON.stringify(VOID_ELEMENTS)};

    function wrapVoidElement(el) {
      const wrapper = document.createElement('span');
      wrapper.className = 'moku-highlight-wrapper';
      wrapper.style.cssText = 'position: relative; display: inline-block;';
      el.parentNode.insertBefore(wrapper, el);
      wrapper.appendChild(el);
      return wrapper;
    }

    function findElement(h) {
      let element = null;
      if (h.type === 'form') {
        const forms = document.getElementsByTagName('form');
        if (h.domIndex !== undefined && forms[h.domIndex]) element = forms[h.domIndex];
      } else if (h.type === 'input') {
        if (h.parentDomIndex !== undefined) {
          const forms = document.getElementsByTagName('form');
          if (forms[h.parentDomIndex]) {
            const inputs = forms[h.parentDomIndex].querySelectorAll('input, textarea, select');
            if (h.domIndex !== undefined && inputs[h.domIndex]) element = inputs[h.domIndex];
          }
        } else {
          const inputs = document.querySelectorAll('input, textarea, select');
          if (h.domIndex !== undefined && inputs[h.domIndex]) element = inputs[h.domIndex];
        }
      } else if (h.type === 'script') {
        const scripts = document.getElementsByTagName('script');
        if (h.domIndex !== undefined && scripts[h.domIndex]) element = scripts[h.domIndex];
      }
      if (!element && h.domId) element = document.getElementById(h.domId);
      if (!element && h.domClasses && h.domClasses.length > 0) {
        const selector = '.' + h.domClasses.join('.');
        try { element = document.querySelector(selector); } catch (_) {}
      }
      return element;
    }

    highlights.forEach((h, idx) => {
      let element = findElement(h);
      if (!element) return;
      const tagName = element.tagName.toLowerCase();
      let targetEl = element;
      if (voidElements.includes(tagName)) targetEl = wrapVoidElement(element);
      targetEl.classList.add('moku-highlight');
      if (idx === activeIndex) targetEl.classList.add('moku-highlight-active');
      if (h.change) {
        if (h.change.kind.includes('added')) targetEl.classList.add('moku-highlight-added');
        else if (h.change.kind.includes('removed')) targetEl.classList.add('moku-highlight-removed');
        else if (h.change.kind.includes('changed')) targetEl.classList.add('moku-highlight-changed');
        targetEl.setAttribute('data-moku-change', h.change.detail || h.change.kind);
      }
    });

    if (activeIndex !== null) {
      const activeEl = document.querySelector('.moku-highlight-active');
      if (activeEl) activeEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  })();
`;

const highlightStyles = `
  .moku-highlight { position: relative !important; border-radius: 6px !important; background-color: rgba(59, 130, 246, 0.08) !important; box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.3), 0 0 12px rgba(59, 130, 246, 0.15) !important; transition: all 0.2s ease !important; cursor: pointer !important; }
  .moku-highlight:hover { background-color: rgba(59, 130, 246, 0.18) !important; box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.6), 0 0 20px rgba(59, 130, 246, 0.3) !important; }
  .moku-highlight-active { box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.7), 0 0 25px rgba(245, 158, 11, 0.4) !important; background-color: rgba(245, 158, 11, 0.15) !important; }
  .moku-highlight-added { background-color: rgba(16, 185, 129, 0.08) !important; box-shadow: 0 0 0 2px rgba(16, 185, 129, 0.3), 0 0 12px rgba(16, 185, 129, 0.15) !important; }
  .moku-highlight-removed { background-color: rgba(239, 68, 68, 0.08) !important; box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.3), 0 0 12px rgba(239, 68, 68, 0.15) !important; }
  .moku-highlight-changed { background-color: rgba(245, 158, 11, 0.08) !important; box-shadow: 0 0 0 2px rgba(245, 158, 11, 0.3), 0 0 12px rgba(245, 158, 11, 0.15) !important; }
  .moku-highlight::after { content: attr(data-moku-change); position: absolute; bottom: calc(100% + 8px); left: 50%; transform: translateX(-50%); background: #0f172a; color: #f1f5f9; font-size: 12px; padding: 8px 10px; border-radius: 6px; white-space: nowrap; z-index: 10000; opacity: 0; visibility: hidden; transition: all 0.2s ease; box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4); }
  .moku-highlight:hover::after { opacity: 1; visibility: visible; }
  .moku-highlights-hidden .moku-highlight, .moku-highlights-hidden .moku-highlight-wrapper { background-color: transparent !important; box-shadow: none !important; cursor: default !important; }
  .moku-highlights-hidden .moku-highlight::after, .moku-highlights-hidden .moku-highlight-wrapper::after { display: none !important; }
  .moku-text-change { border-radius: 2px; padding: 0 2px; margin: 0 -2px; transition: all 0.15s ease; }
  .moku-text-added { background-color: rgba(134, 239, 172, 0.25); border-bottom: 1px dashed rgba(34, 197, 94, 0.5); }
  .moku-text-removed { background-color: rgba(252, 165, 165, 0.25); border-bottom: 1px dashed rgba(239, 68, 68, 0.5); text-decoration: line-through; text-decoration-color: rgba(239, 68, 68, 0.4); }
  .moku-text-changed { background-color: rgba(245, 158, 11, 0.2); border-bottom: 1px solid rgba(245, 158, 11, 0.5); }
  .moku-text-hidden .moku-text-added, .moku-text-hidden .moku-text-removed, .moku-text-hidden .moku-text-changed { background-color: transparent !important; border-bottom: none !important; text-decoration: none !important; }
`;

function injectTextHighlights(html: string, textChanges: TextChange[]): string {
  if (!textChanges || textChanges.length === 0) return html;
  let modifiedHtml = html;

  for (const change of textChanges) {
    if (!change.content || change.content.trim().length < 3) continue;
    const content =
      change.type === "modified" && change.content.includes(" → ")
        ? change.content.split(" → ")[1]
        : change.content;

    const escaped = content
      .trim()
      .replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
      .replace(/\s+/g, "\\s+");
    const className =
      change.type === "added" ? "moku-text-added" : change.type === "removed" ? "moku-text-removed" : "moku-text-changed";
    const pattern = new RegExp(`(>)([^<]*?)(${escaped})([^<]*?)(<)`, "i");
    modifiedHtml = modifiedHtml.replace(pattern, `$1$2<span class="${className}">$3</span>$4$5`);
  }

  return modifiedHtml;
}

export default function RenderedFrame({
  html,
  title,
  highlights = [],
  activeHighlight,
  showHighlights = true,
  textChanges = [],
  showTextHighlights = true,
  className = "",
}: RenderedFrameProps) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [isLoaded, setIsLoaded] = useState(false);

  const preparedHtml = useCallback(() => {
    const baseTag = "<base href=\"about:blank\">";
    const styleTag = `<style>${highlightStyles}</style>`;
    let modified = injectTextHighlights(html, textChanges);

    if (modified.includes("<head>")) {
      modified = modified.replace("<head>", `<head>${baseTag}${styleTag}`);
    } else if (modified.includes("<html>")) {
      modified = modified.replace("<html>", `<html><head>${baseTag}${styleTag}</head>`);
    } else {
      modified = `${baseTag}${styleTag}${modified}`;
    }
    return modified;
  }, [html, textChanges]);

  useEffect(() => {
    if (!iframeRef.current || !isLoaded) return;
    const iframe = iframeRef.current;
    const activeIndex = activeHighlight
      ? highlights.findIndex(
          (item) =>
            item.type === activeHighlight.type &&
            item.domIndex === activeHighlight.domIndex &&
            item.parentDomIndex === activeHighlight.parentDomIndex,
        )
      : null;

    try {
      iframe.contentWindow?.postMessage(
        {
          type: "moku-highlight",
          script: getHighlightScript(highlights, activeIndex, showHighlights, showTextHighlights),
        },
        "*",
      );
    } catch {
      // Ignore cross-origin/sandbox postMessage issues.
    }
  }, [activeHighlight, highlights, isLoaded, showHighlights, showTextHighlights]);

  const handleLoad = useCallback(() => {
    setIsLoaded(true);
    if (!iframeRef.current) return;

    const iframe = iframeRef.current;
    try {
      const doc = iframe.contentDocument;
      if (!doc) return;

      const script = doc.createElement("script");
      script.textContent = `
        window.addEventListener('message', function(e) {
          if (e.data && e.data.type === 'moku-highlight') {
            try { eval(e.data.script); } catch(err) { console.error(err); }
          }
        });
        document.addEventListener('click', function(event) {
          const target = event.target;
          if (!target || !target.closest) return;
          const anchor = target.closest('a[href]');
          if (!anchor) return;
          const href = anchor.getAttribute('href') || '';
          if (href.startsWith('#')) {
            const el = document.querySelector(href);
            if (el && el.scrollIntoView) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
          }
          event.preventDefault();
          event.stopPropagation();
        }, true);
      `;
      doc.body?.appendChild(script);

      const activeIndex = activeHighlight
        ? highlights.findIndex(
            (item) => item.type === activeHighlight.type && item.domIndex === activeHighlight.domIndex,
          )
        : null;

      const highlightScript = doc.createElement("script");
      highlightScript.textContent = getHighlightScript(
        highlights,
        activeIndex,
        showHighlights,
        showTextHighlights,
      );
      doc.body?.appendChild(highlightScript);
    } catch {
      // Ignore cross-origin/sandbox script injection issues.
    }
  }, [activeHighlight, highlights, showHighlights, showTextHighlights]);

  return (
    <div className={`renderedFrame ${className}`}>
      {title && <div className="renderedFrameTitle">{title}</div>}
      <iframe
        ref={iframeRef}
        srcDoc={preparedHtml()}
        sandbox="allow-same-origin allow-scripts"
        onLoad={handleLoad}
        title={title || "Rendered HTML"}
        className="renderedFrameIframe"
      />
    </div>
  );
}
