// DOM Tree representation for the tree view

export type DOMNode = {
  tagName: string
  id?: string
  classes?: string[]
  attributes?: Record<string, string>
  children: DOMNode[]
  textContent?: string
  domIndex?: number  // Index among siblings of same tag type
  changeType?: 'added' | 'removed' | 'changed'
  changeDetail?: string
}

export type ParseOptions = {
  includeText?: boolean
  maxDepth?: number
}

// Parse HTML string into tree structure
export function parseHtmlToTree(html: string, options: ParseOptions = {}): DOMNode | null {
  const { includeText = false, maxDepth = 50 } = options
  
  const parser = new DOMParser()
  const doc = parser.parseFromString(html, 'text/html')
  
  // Track indices for getElementsByTagName equivalence
  const tagCounters: Record<string, number> = {}
  
  function getTagIndex(tagName: string): number {
    const lower = tagName.toLowerCase()
    const idx = tagCounters[lower] || 0
    tagCounters[lower] = idx + 1
    return idx
  }
  
  function parseNode(element: Element, depth: number): DOMNode | null {
    if (depth > maxDepth) return null
    
    const tagName = element.tagName.toLowerCase()
    
    // Skip script and style content for cleaner tree
    if (tagName === 'script' || tagName === 'style') {
      return {
        tagName,
        domIndex: getTagIndex(tagName),
        children: [],
        attributes: getAttributes(element)
      }
    }
    
    const children: DOMNode[] = []
    
    for (const child of element.children) {
      const parsed = parseNode(child, depth + 1)
      if (parsed) children.push(parsed)
    }
    
    // Include text if requested and element has direct text content
    let textContent: string | undefined
    if (includeText) {
      const directText = Array.from(element.childNodes)
        .filter(n => n.nodeType === Node.TEXT_NODE)
        .map(n => n.textContent?.trim())
        .filter(Boolean)
        .join(' ')
      if (directText) textContent = directText.substring(0, 100)
    }
    
    return {
      tagName,
      id: element.id || undefined,
      classes: element.classList.length > 0 ? Array.from(element.classList) : undefined,
      attributes: getAttributes(element),
      children,
      textContent,
      domIndex: getTagIndex(tagName)
    }
  }
  
  function getAttributes(element: Element): Record<string, string> | undefined {
    const attrs: Record<string, string> = {}
    let hasAttrs = false
    
    for (const attr of element.attributes) {
      // Skip common/noisy attributes
      if (attr.name === 'class' || attr.name === 'id') continue
      attrs[attr.name] = attr.value.substring(0, 50)
      hasAttrs = true
    }
    
    return hasAttrs ? attrs : undefined
  }
  
  const bodyNode = parseNode(doc.body, 0)
  return bodyNode
}

// Compare two DOM trees and mark differences
export function diffDomTrees(base: DOMNode | null, head: DOMNode | null): DOMNode | null {
  if (!base && !head) return null
  
  if (!base && head) {
    return markAllAs(head, 'added')
  }
  
  if (base && !head) {
    return markAllAs(base, 'removed')
  }
  
  // Both exist - compare
  return compareNodes(base!, head!)
}

function markAllAs(node: DOMNode, changeType: 'added' | 'removed'): DOMNode {
  return {
    ...node,
    changeType,
    changeDetail: `Element ${changeType}`,
    children: node.children.map(c => markAllAs(c, changeType))
  }
}

function compareNodes(base: DOMNode, head: DOMNode): DOMNode {
  // Check for changes in attributes
  const attrChanged = !sameAttributes(base.attributes, head.attributes)
  const classChanged = !sameArray(base.classes, head.classes)
  const idChanged = base.id !== head.id
  
  let changeType: 'changed' | undefined
  let changeDetail: string | undefined
  
  if (attrChanged || classChanged || idChanged) {
    changeType = 'changed'
    const changes: string[] = []
    if (attrChanged) changes.push('attributes')
    if (classChanged) changes.push('classes')
    if (idChanged) changes.push('id')
    changeDetail = `Changed: ${changes.join(', ')}`
  }
  
  // Compare children by matching tag + index
  const baseChildren = new Map<string, DOMNode>()
  base.children.forEach(c => {
    baseChildren.set(`${c.tagName}-${c.domIndex}`, c)
  })
  
  const headChildren = new Map<string, DOMNode>()
  head.children.forEach(c => {
    headChildren.set(`${c.tagName}-${c.domIndex}`, c)
  })
  
  const resultChildren: DOMNode[] = []
  
  // Process head children (includes added and changed)
  for (const child of head.children) {
    const key = `${child.tagName}-${child.domIndex}`
    const baseChild = baseChildren.get(key)
    
    if (baseChild) {
      resultChildren.push(compareNodes(baseChild, child))
    } else {
      resultChildren.push(markAllAs(child, 'added'))
    }
  }
  
  // Process removed children from base
  for (const child of base.children) {
    const key = `${child.tagName}-${child.domIndex}`
    if (!headChildren.has(key)) {
      resultChildren.push(markAllAs(child, 'removed'))
    }
  }
  
  return {
    ...head,
    changeType,
    changeDetail,
    children: resultChildren
  }
}

function sameAttributes(a?: Record<string, string>, b?: Record<string, string>): boolean {
  if (!a && !b) return true
  if (!a || !b) return false
  
  const keysA = Object.keys(a)
  const keysB = Object.keys(b)
  
  if (keysA.length !== keysB.length) return false
  
  return keysA.every(k => a[k] === b[k])
}

function sameArray(a?: string[], b?: string[]): boolean {
  if (!a && !b) return true
  if (!a || !b) return false
  if (a.length !== b.length) return false
  return a.every((v, i) => v === b[i])
}

// Filter tree to only show changed nodes
export function filterChangedNodes(node: DOMNode): DOMNode | null {
  const hasChange = node.changeType !== undefined
  const filteredChildren = node.children
    .map(c => filterChangedNodes(c))
    .filter((c): c is DOMNode => c !== null)
  
  if (hasChange || filteredChildren.length > 0) {
    return {
      ...node,
      children: filteredChildren
    }
  }
  
  return null
}

// Get summary of changes
export function getChangeSummary(node: DOMNode): { added: number; removed: number; changed: number } {
  let added = 0, removed = 0, changed = 0
  
  function count(n: DOMNode) {
    if (n.changeType === 'added') added++
    else if (n.changeType === 'removed') removed++
    else if (n.changeType === 'changed') changed++
    
    n.children.forEach(count)
  }
  
  count(node)
  return { added, removed, changed }
}
