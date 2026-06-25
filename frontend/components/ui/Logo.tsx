/** The moku mark, drawn inline so it recolors with the active theme
 * (ring/plus in the primary ink, the four nodes in the accent). */
export function Logo({ className = "h-7 w-7" }: { className?: string }) {
  const ink = { stroke: "var(--color-primary)" };
  const node = { fill: "var(--color-accent)" };
  return (
    <svg viewBox="0 0 64 64" className={className} fill="none" aria-hidden="true">
      <circle cx="32" cy="32" r="21" style={ink} strokeWidth="4" />
      <path d="M32 13V51M13 32H51" style={ink} strokeWidth="4" strokeLinecap="round" />
      <circle cx="32" cy="11" r="5" style={node} />
      <circle cx="53" cy="32" r="5" style={node} />
      <circle cx="32" cy="53" r="5" style={node} />
      <circle cx="11" cy="32" r="5" style={node} />
    </svg>
  );
}
