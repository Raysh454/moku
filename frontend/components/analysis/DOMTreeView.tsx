import { useMemo, useState } from "react";
import type { DOMNode } from "./DOMParser";

type DOMTreeViewProps = {
  tree: DOMNode | null;
  showOnlyChanged?: boolean;
  onNodeHover?: (node: DOMNode | null) => void;
  onNodeClick?: (node: DOMNode) => void;
  className?: string;
};

function TreeNode({
  node,
  depth,
  onHover,
  onClick,
  expanded: initialExpanded,
}: {
  node: DOMNode;
  depth: number;
  onHover?: (node: DOMNode | null) => void;
  onClick?: (node: DOMNode) => void;
  expanded: boolean;
}) {
  const [expanded, setExpanded] = useState(initialExpanded);
  const hasChildren = node.children.length > 0;

  const changeClass =
    node.changeType === "added"
      ? "bg-success/10"
      : node.changeType === "removed"
        ? "bg-danger/10"
        : node.changeType === "changed"
          ? "bg-warning/10"
          : "";
  const icon =
    node.changeType === "added"
      ? "+ "
      : node.changeType === "removed"
        ? "- "
        : node.changeType === "changed"
          ? "~ "
          : "";

  const handleClick = (event: React.MouseEvent) => {
    event.stopPropagation();
    if (hasChildren) setExpanded(!expanded);
    onClick?.(node);
  };

  let elementStr = `<${node.tagName}`;
  if (node.id) elementStr += ` id="${node.id}"`;
  if (node.classes?.length) elementStr += ` class="${node.classes.join(" ")}"`;
  if (node.attributes) {
    for (const [key, value] of Object.entries(node.attributes)) {
      const truncated = value.length > 20 ? `${value.substring(0, 20)}...` : value;
      elementStr += ` ${key}="${truncated}"`;
    }
  }
  elementStr += ">";

  return (
    <div className={`py-px ${changeClass}`} style={{ marginLeft: depth * 16 }}>
      <div
        className="flex cursor-pointer items-center gap-0.5 rounded-md px-[5px] py-[3px] hover:bg-white/5"
        onClick={handleClick}
        onMouseEnter={() => onHover?.(node)}
        onMouseLeave={() => onHover?.(null)}
      >
        <span className="w-3 text-muted">{hasChildren ? (expanded ? "▼" : "▶") : " "}</span>
        <span>{icon}</span>
        <span className="text-accent">{elementStr}</span>
        {node.textContent && <span className="italic text-helper"> "{node.textContent.substring(0, 30)}..."</span>}
        {node.changeDetail && <span className="text-warning"> ({node.changeDetail})</span>}
      </div>
      {expanded && hasChildren && (
        <div>
          {node.children.map((child, index) => (
            <TreeNode
              key={`${child.tagName}-${child.domIndex}-${index}`}
              node={child}
              depth={depth + 1}
              onHover={onHover}
              onClick={onClick}
              expanded={depth < 2}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export default function DOMTreeView({
  tree,
  showOnlyChanged = false,
  onNodeHover,
  onNodeClick,
  className = "",
}: DOMTreeViewProps) {
  const displayTree = useMemo(() => {
    if (!tree) return null;
    if (!showOnlyChanged) return tree;

    function filterChanged(node: DOMNode): DOMNode | null {
      const hasChange = node.changeType !== undefined;
      const filteredChildren = node.children
        .map((child) => filterChanged(child))
        .filter((child): child is DOMNode => child !== null);
      if (hasChange || filteredChildren.length > 0) {
        return { ...node, children: filteredChildren };
      }
      return null;
    }

    return filterChanged(tree);
  }, [tree, showOnlyChanged]);

  if (!displayTree) return <div className={`font-mono text-xs ${className}`}>No DOM tree available</div>;

  return (
    <div className={`font-mono text-xs ${className}`}>
      <TreeNode
        node={displayTree}
        depth={0}
        onHover={onNodeHover}
        onClick={onNodeClick}
        expanded={true}
      />
    </div>
  );
}
