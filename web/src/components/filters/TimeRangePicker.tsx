const presets = [
  { label: "15m", minutes: 15 },
  { label: "1h", minutes: 60 },
  { label: "6h", minutes: 360 },
  { label: "24h", minutes: 1440 },
];

function toInputValue(ms: number): string {
  const date = new Date(ms);
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours()
  )}:${pad(date.getMinutes())}`;
}

export function TimeRangePicker({
  startMs,
  endMs,
  onChange,
}: {
  startMs: number;
  endMs: number;
  onChange: (next: { startMs: number; endMs: number }) => void;
}) {
  const applyPreset = (minutes: number) => {
    const end = Date.now();
    const start = end - minutes * 60 * 1000;
    onChange({ startMs: start, endMs: end });
  };

  return (
    <div class="field">
      <label class="field-label">Time range</label>
      <div class="preset-row">
        {presets.map((preset) => (
          <button
            key={preset.label}
            class="preset-button"
            type="button"
            onClick={() => applyPreset(preset.minutes)}
          >
            Last {preset.label}
          </button>
        ))}
      </div>
      <div class="datetime-row">
        <div>
          <span class="field-hint">Start</span>
          <input
            class="field-input"
            type="datetime-local"
            value={toInputValue(startMs)}
            onInput={(event) => {
              const value = (event.target as HTMLInputElement).value;
              const ms = new Date(value).getTime();
              onChange({ startMs: ms, endMs });
            }}
          />
        </div>
        <div>
          <span class="field-hint">End</span>
          <input
            class="field-input"
            type="datetime-local"
            value={toInputValue(endMs)}
            onInput={(event) => {
              const value = (event.target as HTMLInputElement).value;
              const ms = new Date(value).getTime();
              onChange({ startMs, endMs: ms });
            }}
          />
        </div>
      </div>
    </div>
  );
}
