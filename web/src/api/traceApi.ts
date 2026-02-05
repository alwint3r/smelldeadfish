import type { StatusFilter, TraceDetail, TraceQuery, TraceSummary } from "../types";

const API_BASE = "/api";

const defaultHeaders = {
  Accept: "application/json",
};

function buildTraceQueryParams(query: TraceQuery): string {
  const params = new URLSearchParams();
  params.set("service", query.service);
  params.set("start", String(query.start));
  params.set("end", String(query.end));
  params.set("limit", String(query.limit));
  params.set("order", query.order);
  for (const filter of query.attrFilters) {
    if (filter.key.trim() && filter.value.trim()) {
      params.append("attr", `${filter.key}=${filter.value}`);
    }
  }
  if (query.hasError) {
    params.set("has_error", "1");
  }
  return params.toString();
}

export async function fetchTraces(query: TraceQuery, signal?: AbortSignal): Promise<TraceSummary[]> {
  const url = `${API_BASE}/traces?${buildTraceQueryParams(query)}`;
  const response = await fetch(url, { headers: defaultHeaders, signal });
  if (!response.ok) {
    throw new Error(`Trace query failed (${response.status})`);
  }
  const payload = (await response.json()) as { traces?: TraceSummary[] };
  return payload.traces ?? [];
}

export async function fetchTraceDetail(
  traceId: string,
  service?: string,
  status?: StatusFilter,
  signal?: AbortSignal
): Promise<TraceDetail> {
  const params = new URLSearchParams();
  if (service && service.trim()) {
    params.set("service", service);
  }
  if (status && status !== "all") {
    params.set("status", status);
  }
  const query = params.toString();
  const url = query ? `${API_BASE}/traces/${traceId}?${query}` : `${API_BASE}/traces/${traceId}`;
  const response = await fetch(url, { headers: defaultHeaders, signal });
  if (!response.ok) {
    throw new Error(`Trace detail failed (${response.status})`);
  }
  const payload = (await response.json()) as TraceDetail;
  return payload;
}
