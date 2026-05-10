export type DOMNode = {
  tagName: string;
  id?: string;
  classes?: string[];
  attributes?: Record<string, string>;
  children: DOMNode[];
  textContent?: string;
  domIndex?: number;
  changeType?: "added" | "removed" | "changed";
  changeDetail?: string;
};

export type ParseOptions = {
  includeText?: boolean;
  maxDepth?: number;
};

export function parseHtmlToTree(html: string, options: ParseOptions = {}): DOMNode | null {
  const { includeText = false, maxDepth = 50 } = options;

  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");
  const tagCounters: Record<string, number> = {};

  function getTagIndex(tagName: string): number {
    const lower = tagName.toLowerCase();
    const idx = tagCounters[lower] || 0;
    tagCounters[lower] = idx + 1;
    return idx;
  }

  function parseNode(element: Element, depth: number): DOMNode | null {
    if (depth > maxDepth) return null;
    const tagName = element.tagName.toLowerCase();

    if (tagName === "script" || tagName === "style") {
      return {
        tagName,
        domIndex: getTagIndex(tagName),
        children: [],
        attributes: getAttributes(element),
      };
    }

    const children: DOMNode[] = [];
    for (const child of element.children) {
      const parsed = parseNode(child, depth + 1);
      if (parsed) children.push(parsed);
    }

    let textContent: string | undefined;
    if (includeText) {
      const directText = Array.from(element.childNodes)
        .filter((node) => node.nodeType === Node.TEXT_NODE)
        .map((node) => node.textContent?.trim())
        .filter(Boolean)
        .join(" ");
      if (directText) textContent = directText.substring(0, 100);
    }

    return {
      tagName,
      id: element.id || undefined,
      classes: element.classList.length > 0 ? Array.from(element.classList) : undefined,
      attributes: getAttributes(element),
      children,
      textContent,
      domIndex: getTagIndex(tagName),
    };
  }

  function getAttributes(element: Element): Record<string, string> | undefined {
    const attrs: Record<string, string> = {};
    let hasAttrs = false;
    for (const attr of element.attributes) {
      if (attr.name === "class" || attr.name === "id") continue;
      attrs[attr.name] = attr.value.substring(0, 50);
      hasAttrs = true;
    }
    return hasAttrs ? attrs : undefined;
  }

  return parseNode(doc.body, 0);
}

export function diffDomTrees(base: DOMNode | null, head: DOMNode | null): DOMNode | null {
  if (!base && !head) return null;
  if (!base && head) return markAllAs(head, "added");
  if (base && !head) return markAllAs(base, "removed");
  return compareNodes(base!, head!);
}

function markAllAs(node: DOMNode, changeType: "added" | "removed"): DOMNode {
  return {
    ...node,
    changeType,
    changeDetail: `Element ${changeType}`,
    children: node.children.map((child) => markAllAs(child, changeType)),
  };
}

function compareNodes(base: DOMNode, head: DOMNode): DOMNode {
  const attrChanged = !sameAttributes(base.attributes, head.attributes);
  const classChanged = !sameArray(base.classes, head.classes);
  const idChanged = base.id !== head.id;

  let changeType: "changed" | undefined;
  let changeDetail: string | undefined;

  if (attrChanged || classChanged || idChanged) {
    changeType = "changed";
    const changes: string[] = [];
    if (attrChanged) changes.push("attributes");
    if (classChanged) changes.push("classes");
    if (idChanged) changes.push("id");
    changeDetail = `Changed: ${changes.join(", ")}`;
  }

  const baseChildren = new Map<string, DOMNode>();
  base.children.forEach((child) => {
    baseChildren.set(`${child.tagName}-${child.domIndex}`, child);
  });

  const headChildren = new Map<string, DOMNode>();
  head.children.forEach((child) => {
    headChildren.set(`${child.tagName}-${child.domIndex}`, child);
  });

  const resultChildren: DOMNode[] = [];
  for (const child of head.children) {
    const key = `${child.tagName}-${child.domIndex}`;
    const baseChild = baseChildren.get(key);
    if (baseChild) {
      resultChildren.push(compareNodes(baseChild, child));
    } else {
      resultChildren.push(markAllAs(child, "added"));
    }
  }

  for (const child of base.children) {
    const key = `${child.tagName}-${child.domIndex}`;
    if (!headChildren.has(key)) {
      resultChildren.push(markAllAs(child, "removed"));
    }
  }

  return {
    ...head,
    changeType,
    changeDetail,
    children: resultChildren,
  };
}

function sameAttributes(a?: Record<string, string>, b?: Record<string, string>): boolean {
  if (!a && !b) return true;
  if (!a || !b) return false;

  const keysA = Object.keys(a);
  const keysB = Object.keys(b);
  if (keysA.length !== keysB.length) return false;
  return keysA.every((key) => a[key] === b[key]);
}

function sameArray(a?: string[], b?: string[]): boolean {
  if (!a && !b) return true;
  if (!a || !b) return false;
  if (a.length !== b.length) return false;
  return a.every((value, index) => value === b[index]);
}

export function getChangeSummary(node: DOMNode): { added: number; removed: number; changed: number } {
  let added = 0;
  let removed = 0;
  let changed = 0;

  function count(current: DOMNode) {
    if (current.changeType === "added") added += 1;
    else if (current.changeType === "removed") removed += 1;
    else if (current.changeType === "changed") changed += 1;
    current.children.forEach(count);
  }

  count(node);
  return { added, removed, changed };
}
