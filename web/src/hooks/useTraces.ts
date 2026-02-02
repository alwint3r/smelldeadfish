import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import { fetchTraces } from "../api/traceApi";
import type { AttrFilter, TraceQuery, TraceSummary } from "../types";

export type TraceQueryState = {
  status: "idle" | "loading" | "success" | "error";
  data: TraceSummary[];
  error?: string;
};

export function useTraces(query: TraceQuery | null): TraceQueryState {
  const [state, setState] = useState<TraceQueryState>({
    status: "idle",
    data: [],
  });
  const previousQueryRef = useRef<TraceQuery | null>(null);

  const queryKey = useMemo(() => (query ? JSON.stringify(query) : ""), [query]);

  useEffect(() => {
    if (!query) {
      setState({ status: "idle", data: [] });
      return;
    }

    const controller = new AbortController();
    const previous = previousQueryRef.current;
    const isLoadMore = isLoadMoreQuery(previous, query);
    setState((current) => ({
      status: "loading",
      data: isLoadMore ? current.data : [],
    }));

    fetchTraces(query, controller.signal)
      .then((traces) => {
        setState({ status: "success", data: traces });
        previousQueryRef.current = query;
      })
      .catch((error: Error) => {
        if (controller.signal.aborted) {
          return;
        }
        setState({ status: "error", data: [], error: error.message });
      });

    return () => {
      controller.abort();
    };
  }, [queryKey]);

  return state;
}

function isLoadMoreQuery(previous: TraceQuery | null, next: TraceQuery): boolean {
  if (!previous) {
    return false;
  }
  if (previous.service !== next.service) {
    return false;
  }
  if (previous.start !== next.start || previous.end !== next.end) {
    return false;
  }
  if (previous.order !== next.order) {
    return false;
  }
  if (previous.limit >= next.limit) {
    return false;
  }
  return sameFilters(previous.attrFilters, next.attrFilters);
}

function sameFilters(left: AttrFilter[], right: AttrFilter[]): boolean {
  if (left.length !== right.length) {
    return false;
  }
  for (let i = 0; i < left.length; i += 1) {
    if (left[i].key !== right[i].key || left[i].value !== right[i].value) {
      return false;
    }
  }
  return true;
}
