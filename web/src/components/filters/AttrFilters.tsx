import type { AttrFilter } from "../../types";

export function AttrFilters({
  filters,
  onChange,
}: {
  filters: AttrFilter[];
  onChange: (filters: AttrFilter[]) => void;
}) {
  const updateFilter = (index: number, patch: Partial<AttrFilter>) => {
    const next = filters.map((filter, idx) =>
      idx === index ? { ...filter, ...patch } : filter
    );
    onChange(next);
  };

  const addFilter = () => {
    onChange([...filters, { key: "", value: "" }]);
  };

  const removeFilter = (index: number) => {
    onChange(filters.filter((_, idx) => idx !== index));
  };

  return (
    <div class="field">
      <label class="field-label">Attribute filters</label>
      {filters.length === 0 ? (
        <div class="empty-state">No attribute filters</div>
      ) : null}
      <div class="attr-list">
        {filters.map((filter, index) => (
          <div class="attr-row" key={`${filter.key}-${index}`}>
            <input
              class="field-input"
              type="text"
              placeholder="key"
              value={filter.key}
              onInput={(event) =>
                updateFilter(index, {
                  key: (event.target as HTMLInputElement).value,
                })
              }
            />
            <input
              class="field-input"
              type="text"
              placeholder="value"
              value={filter.value}
              onInput={(event) =>
                updateFilter(index, {
                  value: (event.target as HTMLInputElement).value,
                })
              }
            />
            <button class="ghost-button" type="button" onClick={() => removeFilter(index)}>
              Remove
            </button>
          </div>
        ))}
      </div>
      <button class="secondary-button" type="button" onClick={addFilter}>
        Add filter
      </button>
    </div>
  );
}
