import { useMemo, useState } from "preact/hooks";
import { ServiceSelect } from "../components/filters/ServiceSelect";
import { TimeRangePicker } from "../components/filters/TimeRangePicker";
import { AttrFilters } from "../components/filters/AttrFilters";
import { TraceList } from "../components/trace/TraceList";
import { TraceSummaryBar } from "../components/trace/TraceSummaryBar";
import { useTraces } from "../hooks/useTraces";
import type { AttrFilter, TraceQuery } from "../types";

const DEFAULT_LIMIT = 100;
const LIMIT_STEP = 100;

export function SearchPage() {
  const now = Date.now();
  const [service, setService] = useState("");
  const [serviceError, setServiceError] = useState<string | undefined>(undefined);
  const [filters, setFilters] = useState<AttrFilter[]>([]);
  const [range, setRange] = useState({
    startMs: now - 60 * 60 * 1000,
    endMs: now,
  });
  const [limit, setLimit] = useState(DEFAULT_LIMIT);
  const [query, setQuery] = useState<TraceQuery | null>(null);

  const sanitizedFilters = useMemo(
    () => filters.filter((filter) => filter.key.trim() && filter.value.trim()),
    [filters]
  );

  const traceState = useTraces(query);

  const handleSearch = () => {
    if (!service.trim()) {
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
    const startNs = Math.floor(range.startMs * 1_000_000);
    const endNs = Math.floor(range.endMs * 1_000_000);
    setQuery({
      service: service.trim(),
      start: startNs,
      end: endNs,
      limit: nextLimit,
      attrFilters: sanitizedFilters,
    });
  };

  const handleLoadMore = () => {
    if (!query) {
      return;
    }
    const nextLimit = limit + LIMIT_STEP;
    setLimit(nextLimit);
    setQuery({ ...query, limit: nextLimit });
  };

  return (
    <div class="layout">
      <aside class="filters-panel">
        <div class="panel-title">Search</div>
        <ServiceSelect value={service} onChange={setService} error={serviceError} />
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
        />
      </section>
    </div>
  );
}
