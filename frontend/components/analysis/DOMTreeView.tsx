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

  const changeClass = node.changeType ? `treeNode-${node.changeType}` : "";
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
    <div className={`treeNodeContainer ${changeClass}`} style={{ marginLeft: depth * 16 }}>
      <div
        className="treeNodeRow"
        onClick={handleClick}
        onMouseEnter={() => onHover?.(node)}
        onMouseLeave={() => onHover?.(null)}
      >
        <span className="treeNodeExpander">{hasChildren ? (expanded ? "▼" : "▶") : " "}</span>
        <span className="treeNodeIcon">{icon}</span>
        <span className="treeNodeTag">{elementStr}</span>
        {node.textContent && <span className="treeNodeText"> "{node.textContent.substring(0, 30)}..."</span>}
        {node.changeDetail && <span className="treeNodeChange"> ({node.changeDetail})</span>}
      </div>
      {expanded && hasChildren && (
        <div className="treeNodeChildren">
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

  if (!displayTree) return <div className={`domTreeView ${className}`}>No DOM tree available</div>;

  return (
    <div className={`domTreeView ${className}`}>
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
