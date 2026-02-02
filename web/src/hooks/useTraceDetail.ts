import { useEffect, useMemo, useState } from "preact/hooks";
import { fetchTraceDetail } from "../api/traceApi";
import type { TraceDetail } from "../types";

export type TraceDetailState = {
  status: "idle" | "loading" | "success" | "error";
  data?: TraceDetail;
  error?: string;
};

export function useTraceDetail(traceId: string | undefined): TraceDetailState {
  const [state, setState] = useState<TraceDetailState>({ status: "idle" });
  const key = useMemo(() => traceId ?? "", [traceId]);

  useEffect(() => {
    if (!traceId) {
      setState({ status: "idle" });
      return;
    }

    const controller = new AbortController();
    setState({ status: "loading" });

    fetchTraceDetail(traceId, undefined, controller.signal)
      .then((detail) => {
        setState({ status: "success", data: detail });
      })
      .catch((error: Error) => {
        if (controller.signal.aborted) {
          return;
        }
        setState({ status: "error", error: error.message });
      });

    return () => {
      controller.abort();
    };
  }, [key]);

  return state;
}
