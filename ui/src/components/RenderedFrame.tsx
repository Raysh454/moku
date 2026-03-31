import { useEffect, useRef, useCallback, useState } from 'react'
import type { AttackSurfaceChange } from '../api/types'

export type HighlightedElement = {
  type: string
  domIndex?: number
  parentDomIndex?: number
  domId?: string
  domClasses?: string[]
  change?: AttackSurfaceChange
}

// Text change from body_diff
export type TextChange = {
  type: 'added' | 'removed' | 'modified'
  content: string
}

type RenderedFrameProps = {
  html: string
  title?: string
  highlights?: HighlightedElement[]
  activeHighlight?: HighlightedElement | null
  showHighlights?: boolean
  textChanges?: TextChange[]
  showTextHighlights?: boolean
  onElementHover?: (element: HighlightedElement | null) => void
  onElementClick?: (element: HighlightedElement) => void
  className?: string
}

// List of void elements that cannot have ::after pseudo-elements
const VOID_ELEMENTS = ['input', 'br', 'hr', 'img', 'meta', 'link', 'area', 'base', 'col', 'embed', 'param', 'source', 'track', 'wbr']

// Script to inject into iframe for highlighting elements
const getHighlightScript = (highlights: HighlightedElement[], activeIndex: number | null, showHighlights: boolean, showTextHighlights: boolean) => `
  (function() {
    // Toggle highlight visibility classes on body
    document.body.classList.toggle('moku-highlights-hidden', !${showHighlights});
    document.body.classList.toggle('moku-text-hidden', !${showTextHighlights});
    
    // Remove existing highlight wrappers and restore original elements
    document.querySelectorAll('.moku-highlight-wrapper').forEach(wrapper => {
      const original = wrapper.firstChild;
      if (original) {
        wrapper.parentNode.replaceChild(original, wrapper);
      }
    });
    
    // Remove existing highlight classes
    document.querySelectorAll('.moku-highlight').forEach(el => {
      el.classList.remove('moku-highlight', 'moku-highlight-active', 'moku-highlight-added', 'moku-highlight-removed', 'moku-highlight-changed');
      el.removeAttribute('data-moku-change');
    });

    const highlights = ${JSON.stringify(highlights)};
    const activeIndex = ${activeIndex};
    const voidElements = ${JSON.stringify(VOID_ELEMENTS)};

    // Helper: wrap void element in a span for pseudo-element support
    function wrapVoidElement(el) {
      const wrapper = document.createElement('span');
      wrapper.className = 'moku-highlight-wrapper';
      wrapper.style.cssText = 'position: relative; display: inline-block;';
      el.parentNode.insertBefore(wrapper, el);
      wrapper.appendChild(el);
      return wrapper;
    }

    // Helper: find element with fallback to ID/class
    function findElement(h) {
      let element = null;
      
      if (h.type === 'form') {
        const forms = document.getElementsByTagName('form');
        if (h.domIndex !== undefined && forms[h.domIndex]) {
          element = forms[h.domIndex];
        }
      } else if (h.type === 'input') {
        // Fix: Query input, textarea, AND select (matches backend indexing)
        if (h.parentDomIndex !== undefined) {
          const forms = document.getElementsByTagName('form');
          if (forms[h.parentDomIndex]) {
            const inputs = forms[h.parentDomIndex].querySelectorAll('input, textarea, select');
            if (h.domIndex !== undefined && inputs[h.domIndex]) {
              element = inputs[h.domIndex];
            }
          }
        } else {
          const inputs = document.querySelectorAll('input, textarea, select');
          if (h.domIndex !== undefined && inputs[h.domIndex]) {
            element = inputs[h.domIndex];
          }
        }
      } else if (h.type === 'script') {
        const scripts = document.getElementsByTagName('script');
        if (h.domIndex !== undefined && scripts[h.domIndex]) {
          element = scripts[h.domIndex];
        }
      }
      
      // Fallback: try ID if available
      if (!element && h.domId) {
        element = document.getElementById(h.domId);
      }
      
      // Fallback: try classes if available
      if (!element && h.domClasses && h.domClasses.length > 0) {
        const selector = '.' + h.domClasses.join('.');
        try {
          element = document.querySelector(selector);
        } catch (e) {
          // Invalid selector, skip
        }
      }
      
      return element;
    }

    highlights.forEach((h, idx) => {
      let element = findElement(h);

      if (element) {
        const tagName = element.tagName.toLowerCase();
        let targetEl = element;
        
        // Wrap void elements so ::after popup can work
        if (voidElements.includes(tagName)) {
          targetEl = wrapVoidElement(element);
        }
        
        targetEl.classList.add('moku-highlight');
        if (idx === activeIndex) {
          targetEl.classList.add('moku-highlight-active');
        }
        
        // Add change type class
        if (h.change) {
          if (h.change.kind.includes('added')) {
            targetEl.classList.add('moku-highlight-added');
          } else if (h.change.kind.includes('removed')) {
            targetEl.classList.add('moku-highlight-removed');
          } else if (h.change.kind.includes('changed')) {
            targetEl.classList.add('moku-highlight-changed');
          }
          targetEl.setAttribute('data-moku-change', h.change.detail || h.change.kind);
        }
      }
    });

    // Scroll to active element
    if (activeIndex !== null) {
      const activeEl = document.querySelector('.moku-highlight-active');
      if (activeEl) {
        activeEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    }
  })();
`

// CSS to inject for highlights - prettier rounded transparent design
const highlightStyles = `
  .moku-highlight {
    position: relative !important;
    border-radius: 6px !important;
    background-color: rgba(59, 130, 246, 0.08) !important;
    box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.3), 0 0 12px rgba(59, 130, 246, 0.15) !important;
    transition: all 0.2s ease !important;
    cursor: pointer !important;
  }
  .moku-highlight:hover {
    background-color: rgba(59, 130, 246, 0.18) !important;
    box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.6), 0 0 20px rgba(59, 130, 246, 0.3) !important;
  }
  .moku-highlight-active {
    box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.7), 0 0 25px rgba(245, 158, 11, 0.4) !important;
    background-color: rgba(245, 158, 11, 0.15) !important;
    animation: moku-glow 1.5s ease-in-out infinite alternate;
  }
  .moku-highlight-added {
    background-color: rgba(16, 185, 129, 0.08) !important;
    box-shadow: 0 0 0 2px rgba(16, 185, 129, 0.3), 0 0 12px rgba(16, 185, 129, 0.15) !important;
  }
  .moku-highlight-added:hover {
    background-color: rgba(16, 185, 129, 0.2) !important;
    box-shadow: 0 0 0 2px rgba(16, 185, 129, 0.6), 0 0 20px rgba(16, 185, 129, 0.3) !important;
  }
  .moku-highlight-removed {
    background-color: rgba(239, 68, 68, 0.08) !important;
    box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.3), 0 0 12px rgba(239, 68, 68, 0.15) !important;
  }
  .moku-highlight-removed:hover {
    background-color: rgba(239, 68, 68, 0.2) !important;
    box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.6), 0 0 20px rgba(239, 68, 68, 0.3) !important;
  }
  .moku-highlight-changed {
    background-color: rgba(245, 158, 11, 0.08) !important;
    box-shadow: 0 0 0 2px rgba(245, 158, 11, 0.3), 0 0 12px rgba(245, 158, 11, 0.15) !important;
  }
  .moku-highlight-changed:hover {
    background-color: rgba(245, 158, 11, 0.2) !important;
    box-shadow: 0 0 0 2px rgba(245, 158, 11, 0.6), 0 0 20px rgba(245, 158, 11, 0.3) !important;
  }
  @keyframes moku-glow {
    from { 
      box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.7), 0 0 20px rgba(245, 158, 11, 0.3);
    }
    to { 
      box-shadow: 0 0 0 4px rgba(245, 158, 11, 0.5), 0 0 30px rgba(245, 158, 11, 0.5);
    }
  }
  /* Popup tooltip card */
  .moku-highlight::after {
    content: attr(data-moku-change);
    position: absolute;
    bottom: calc(100% + 8px);
    left: 50%;
    transform: translateX(-50%) translateY(5px);
    background: linear-gradient(135deg, #1e293b 0%, #0f172a 100%);
    color: #f1f5f9;
    font-size: 12px;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    padding: 8px 12px;
    border-radius: 8px;
    white-space: nowrap;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    z-index: 10000;
    pointer-events: none;
    opacity: 0;
    visibility: hidden;
    transition: all 0.2s ease;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.1);
    backdrop-filter: blur(8px);
  }
  .moku-highlight::before {
    content: '';
    position: absolute;
    bottom: calc(100% + 4px);
    left: 50%;
    transform: translateX(-50%) translateY(5px);
    border: 6px solid transparent;
    border-top-color: #1e293b;
    z-index: 10001;
    opacity: 0;
    visibility: hidden;
    transition: all 0.2s ease;
  }
  .moku-highlight:hover::after,
  .moku-highlight:hover::before {
    opacity: 1;
    visibility: visible;
    transform: translateX(-50%) translateY(0);
  }
  /* Badge styling within tooltip */
  .moku-highlight-added::after {
    border-left: 3px solid #10b981;
  }
  .moku-highlight-removed::after {
    border-left: 3px solid #ef4444;
  }
  .moku-highlight-changed::after {
    border-left: 3px solid #f59e0b;
  }
  /* Hidden state for toggle */
  .moku-highlights-hidden .moku-highlight,
  .moku-highlights-hidden .moku-highlight-wrapper {
    background-color: transparent !important;
    box-shadow: none !important;
    cursor: default !important;
  }
  .moku-highlights-hidden .moku-highlight::after,
  .moku-highlights-hidden .moku-highlight::before,
  .moku-highlights-hidden .moku-highlight-wrapper::after,
  .moku-highlights-hidden .moku-highlight-wrapper::before {
    display: none !important;
  }
  
  /* Text change highlights - subtle grey style */
  .moku-text-change {
    border-radius: 2px;
    padding: 0 2px;
    margin: 0 -2px;
    transition: all 0.15s ease;
  }
  .moku-text-added {
    background-color: rgba(134, 239, 172, 0.25);
    border-bottom: 1px dashed rgba(34, 197, 94, 0.5);
  }
  .moku-text-added:hover {
    background-color: rgba(134, 239, 172, 0.4);
  }
  .moku-text-removed {
    background-color: rgba(252, 165, 165, 0.25);
    border-bottom: 1px dashed rgba(239, 68, 68, 0.5);
    text-decoration: line-through;
    text-decoration-color: rgba(239, 68, 68, 0.4);
  }
  .moku-text-removed:hover {
    background-color: rgba(252, 165, 165, 0.4);
  }
  .moku-text-changed {
    background-color: rgba(245, 158, 11, 0.2);
    border-bottom: 1px solid rgba(245, 158, 11, 0.5);
  }
  .moku-text-changed:hover {
    background-color: rgba(245, 158, 11, 0.35);
  }
  .moku-text-neutral {
    background-color: rgba(156, 163, 175, 0.15);
    border-bottom: 1px dotted rgba(107, 114, 128, 0.4);
  }
  .moku-text-neutral:hover {
    background-color: rgba(156, 163, 175, 0.3);
  }
  
  /* Hidden state for text highlights */
  .moku-text-hidden .moku-text-change {
    background-color: transparent !important;
    border-bottom: none !important;
    text-decoration: none !important;
  }
`

// Function to inject text change highlights into HTML
function injectTextHighlights(html: string, textChanges: TextChange[]): string {
  if (!textChanges || textChanges.length === 0) return html
  
  let modifiedHtml = html
  
  // Process each text change
  for (const change of textChanges) {
    if (!change.content || change.content.length < 3) continue // Skip very short changes
    
    // Escape special regex characters in content
    const escapedContent = change.content.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    
    // Only highlight if content appears in HTML (outside of tags)
    // This is a simplified approach - matches text content, not inside attributes
    const textPattern = new RegExp(
      `(>)([^<]*?)(${escapedContent})([^<]*?)(<)`,
      'g'
    )
    
    const className = change.type === 'added' ? 'moku-text-added' : 
                      change.type === 'removed' ? 'moku-text-removed' : 
                      change.type === 'modified' ? 'moku-text-changed' :
                      'moku-text-neutral'
    
    modifiedHtml = modifiedHtml.replace(
      textPattern,
      `$1$2<span class="moku-text-change ${className}">$3</span>$4$5`
    )
  }
  
  return modifiedHtml
}

export default function RenderedFrame({
  html,
  title,
  highlights = [],
  activeHighlight,
  showHighlights = true,
  textChanges = [],
  showTextHighlights = true,
  onElementHover: _onElementHover,
  onElementClick: _onElementClick,
  className = ''
}: RenderedFrameProps) {
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const [isLoaded, setIsLoaded] = useState(false)

  // Prepare HTML with injected styles and text highlights
  const preparedHtml = useCallback(() => {
    // Add a base tag to prevent external resource loading
    const baseTag = '<base href="about:blank">'
    const styleTag = `<style>${highlightStyles}</style>`
    
    // Inject text change highlights into HTML
    let modified = injectTextHighlights(html, textChanges)
    
    // Insert into head or at beginning of document
    if (modified.includes('<head>')) {
      modified = modified.replace('<head>', `<head>${baseTag}${styleTag}`)
    } else if (modified.includes('<html>')) {
      modified = modified.replace('<html>', `<html><head>${baseTag}${styleTag}</head>`)
    } else {
      modified = `${baseTag}${styleTag}${modified}`
    }
    
    return modified
  }, [html, textChanges])

  // Update highlights when they change
  useEffect(() => {
    if (!iframeRef.current || !isLoaded) return
    
    const iframe = iframeRef.current
    const activeIndex = activeHighlight 
      ? highlights.findIndex(h => 
          h.type === activeHighlight.type && 
          h.domIndex === activeHighlight.domIndex &&
          h.parentDomIndex === activeHighlight.parentDomIndex
        )
      : null

    try {
      iframe.contentWindow?.postMessage({
        type: 'moku-highlight',
        script: getHighlightScript(highlights, activeIndex, showHighlights, showTextHighlights)
      }, '*')
    } catch {
      // Cross-origin restriction - expected with sandbox
    }
  }, [highlights, activeHighlight, isLoaded, showHighlights, showTextHighlights])

  // Handle iframe load
  const handleLoad = useCallback(() => {
    setIsLoaded(true)
    
    if (!iframeRef.current) return
    const iframe = iframeRef.current
    
    try {
      const doc = iframe.contentDocument
      if (!doc) return

      // Inject message handler for highlight updates
      const script = doc.createElement('script')
      script.textContent = `
        window.addEventListener('message', function(e) {
          if (e.data && e.data.type === 'moku-highlight') {
            try { eval(e.data.script); } catch(err) { console.error(err); }
          }
        });
      `
      doc.body?.appendChild(script)

      // Initial highlight injection
      const activeIndex = activeHighlight 
        ? highlights.findIndex(h => 
            h.type === activeHighlight.type && 
            h.domIndex === activeHighlight.domIndex
          )
        : null
      
      const highlightScript = doc.createElement('script')
      highlightScript.textContent = getHighlightScript(highlights, activeIndex, showHighlights, showTextHighlights)
      doc.body?.appendChild(highlightScript)

    } catch {
      // Sandbox restriction - expected
    }
  }, [highlights, activeHighlight, showHighlights, showTextHighlights])

  return (
    <div className={`renderedFrame ${className}`}>
      {title && <div className="renderedFrameTitle">{title}</div>}
      <iframe
        ref={iframeRef}
        srcDoc={preparedHtml()}
        sandbox="allow-same-origin allow-scripts"
        onLoad={handleLoad}
        title={title || 'Rendered HTML'}
        className="renderedFrameIframe"
      />
    </div>
  )
}
