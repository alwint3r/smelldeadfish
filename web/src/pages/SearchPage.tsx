import { useEffect, useMemo, useState } from "preact/hooks";
import { ServiceSelect } from "../components/filters/ServiceSelect";
import { TimeRangePicker } from "../components/filters/TimeRangePicker";
import { AttrFilters } from "../components/filters/AttrFilters";
import { TraceList } from "../components/trace/TraceList";
import { TraceSummaryBar } from "../components/trace/TraceSummaryBar";
import { useTraces } from "../hooks/useTraces";
import type { AttrFilter, TraceOrder, TraceQuery } from "../types";

const DEFAULT_LIMIT = 100;
const LIMIT_STEP = 100;
const DEFAULT_RANGE_MS = 60 * 60 * 1000;
const MS_TO_NS = 1_000_000;
const DEFAULT_ORDER: TraceOrder = "start_desc";

const ORDER_OPTIONS: Array<{ value: TraceOrder; label: string }> = [
  { value: "start_desc", label: "Newest first" },
  { value: "start_asc", label: "Oldest first" },
  { value: "duration_desc", label: "Duration: longest first" },
  { value: "duration_asc", label: "Duration: shortest first" },
];

type UrlSearchState = {
  service: string;
  startMs: number;
  endMs: number;
  limit: number;
  order: TraceOrder;
};

function getDefaultRange(): { startMs: number; endMs: number } {
  const now = Date.now();
  return { startMs: now - DEFAULT_RANGE_MS, endMs: now };
}

function toNs(ms: number): number {
  return Math.floor(ms * MS_TO_NS);
}

function toMs(ns: number): number {
  return Math.floor(ns / MS_TO_NS);
}

function parseNumberParam(params: URLSearchParams, key: string): number | null {
  const raw = params.get(key);
  if (!raw) {
    return null;
  }
  const value = Number(raw);
  if (!Number.isFinite(value)) {
    return null;
  }
  return Math.trunc(value);
}

function parseOrderParam(params: URLSearchParams): TraceOrder {
  const raw = (params.get("order") ?? "").trim();
  switch (raw) {
    case "start_desc":
    case "start_asc":
    case "duration_desc":
    case "duration_asc":
      return raw;
    default:
      return DEFAULT_ORDER;
  }
}

function parseSearchState(search: string): UrlSearchState {
  const params = new URLSearchParams(search);
  const defaults = getDefaultRange();
  const service = (params.get("service") ?? "").trim();

  let startMs = parseNumberParam(params, "startMs");
  let endMs = parseNumberParam(params, "endMs");
  let limit = parseNumberParam(params, "limit");
  const order = parseOrderParam(params);

  if (startMs === null) {
    startMs = defaults.startMs;
  }
  if (endMs === null) {
    endMs = defaults.endMs;
  }
  if (endMs < startMs) {
    startMs = defaults.startMs;
    endMs = defaults.endMs;
  }
  if (limit === null || limit <= 0) {
    limit = DEFAULT_LIMIT;
  }

  return { service, startMs, endMs, limit, order };
}

function buildSearchParams(state: UrlSearchState): string {
  const service = state.service.trim();
  if (!service) {
    return "";
  }
  const params = new URLSearchParams();
  params.set("service", service);
  params.set("startMs", String(Math.trunc(state.startMs)));
  params.set("endMs", String(Math.trunc(state.endMs)));
  params.set("limit", String(Math.trunc(state.limit)));
  params.set("order", state.order);
  return params.toString();
}

export function SearchPage() {
  const [service, setService] = useState("");
  const [serviceError, setServiceError] = useState<string | undefined>(undefined);
  const [filters, setFilters] = useState<AttrFilter[]>([]);
  const [range, setRange] = useState(() => getDefaultRange());
  const [limit, setLimit] = useState(DEFAULT_LIMIT);
  const [order, setOrder] = useState<TraceOrder>(DEFAULT_ORDER);
  const [query, setQuery] = useState<TraceQuery | null>(null);

  const sanitizedFilters = useMemo(
    () => filters.filter((filter) => filter.key.trim() && filter.value.trim()),
    [filters]
  );

  const traceState = useTraces(query);
  const searchSuffix = window.location.search;

  const applyUrlState = (search: string) => {
    const parsed = parseSearchState(search);
    setService(parsed.service);
    setRange({ startMs: parsed.startMs, endMs: parsed.endMs });
    setLimit(parsed.limit);
    setOrder(parsed.order);
    setFilters([]);
    setServiceError(undefined);

    if (!parsed.service) {
      setQuery(null);
      return;
    }

    setQuery({
      service: parsed.service,
      start: toNs(parsed.startMs),
      end: toNs(parsed.endMs),
      limit: parsed.limit,
      order: parsed.order,
      attrFilters: [],
    });
  };

  useEffect(() => {
    applyUrlState(window.location.search);

    const handlePopState = () => {
      applyUrlState(window.location.search);
    };

    window.addEventListener("popstate", handlePopState);
    return () => {
      window.removeEventListener("popstate", handlePopState);
    };
  }, []);

  const handleSearch = () => {
    const trimmedService = service.trim();
    if (!trimmedService) {
      setServiceError("Service is required.");
      return;
    }
    setServiceError(undefined);
    if (range.endMs < range.startMs) {
      setServiceError("End time must be after start time.");
      return;
    }
    const nextLimit = DEFAULT_LIMIT;
    setLimit(nextLimit);
    const startNs = toNs(range.startMs);
    const endNs = toNs(range.endMs);
    setQuery({
      service: trimmedService,
      start: startNs,
      end: endNs,
      limit: nextLimit,
      order,
      attrFilters: sanitizedFilters,
    });

    const searchParams = buildSearchParams({
      service: trimmedService,
      startMs: range.startMs,
      endMs: range.endMs,
      limit: nextLimit,
      order,
    });
    const nextUrl = searchParams ? `${window.location.pathname}?${searchParams}` : window.location.pathname;
    window.history.pushState(null, "", nextUrl);
  };

  const handleLoadMore = () => {
    if (!query) {
      return;
    }
    const nextLimit = limit + LIMIT_STEP;
    setLimit(nextLimit);
    setQuery({ ...query, limit: nextLimit });

    const searchParams = buildSearchParams({
      service: query.service,
      startMs: toMs(query.start),
      endMs: toMs(query.end),
      limit: nextLimit,
      order: query.order,
    });
    const nextUrl = searchParams ? `${window.location.pathname}?${searchParams}` : window.location.pathname;
    window.history.pushState(null, "", nextUrl);
  };

  return (
    <div class="layout">
      <aside class="filters-panel">
        <div class="panel-title">Search</div>
        <ServiceSelect value={service} onChange={setService} error={serviceError} />
        <div class="field">
          <label class="field-label" for="order-input">
            Order
          </label>
          <select
            id="order-input"
            class="field-input"
            value={order}
            onChange={(event) => setOrder((event.target as HTMLSelectElement).value as TraceOrder)}
          >
            {ORDER_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <div class="field-hint">Sort traces by start time or duration.</div>
        </div>
        <TimeRangePicker startMs={range.startMs} endMs={range.endMs} onChange={setRange} />
        <AttrFilters filters={filters} onChange={setFilters} />
        <button class="primary-button" type="button" onClick={handleSearch}>
          Search traces
        </button>
      </aside>
      <section class="results-panel">
        <div class="results-header">
          <div>
            <h1>Traces</h1>
            <p class="muted">
              Search results for the selected service and time range.
            </p>
          </div>
        </div>
        {query ? (
          <TraceSummaryBar
            count={traceState.data.length}
            startNs={query.start}
            endNs={query.end}
          />
        ) : null}
        <TraceList
          traces={traceState.data}
          status={traceState.status}
          error={traceState.error}
          onLoadMore={handleLoadMore}
          canLoadMore={
            traceState.status === "success" && !!query && traceState.data.length >= query.limit
          }
          searchSuffix={searchSuffix}
        />
      </section>
    </div>
  );
}
