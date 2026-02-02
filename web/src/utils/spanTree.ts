import type { Span } from "../types";

export type SpanNode = {
  span: Span;
  children: SpanNode[];
  depth: number;
};

const ROOT_PARENT_ID = "0000000000000000";

export function buildSpanTree(spans: Span[]): SpanNode[] {
  const nodes = new Map<string, SpanNode>();
  for (const span of spans) {
    nodes.set(span.span_id, { span, children: [], depth: 0 });
  }

  const roots: SpanNode[] = [];
  for (const node of nodes.values()) {
    const parentId = node.span.parent_span_id;
    if (!parentId || parentId === ROOT_PARENT_ID) {
      roots.push(node);
      continue;
    }
    const parent = nodes.get(parentId);
    if (parent) {
      parent.children.push(node);
    } else {
      roots.push(node);
    }
  }

  const sortTree = (list: SpanNode[]) => {
    list.sort((a, b) => a.span.start_time_unix_nano - b.span.start_time_unix_nano);
    for (const node of list) {
      sortTree(node.children);
    }
  };
  sortTree(roots);

  const assignDepth = (list: SpanNode[], depth: number) => {
    for (const node of list) {
      node.depth = depth;
      assignDepth(node.children, depth + 1);
    }
  };
  assignDepth(roots, 0);

  return roots;
}

export function flattenTree(nodes: SpanNode[]): SpanNode[] {
  const flat: SpanNode[] = [];
  const walk = (list: SpanNode[]) => {
    for (const node of list) {
      flat.push(node);
      walk(node.children);
    }
  };
  walk(nodes);
  return flat;
}
