export function ServiceSelect({
  value,
  onChange,
  error,
}: {
  value: string;
  onChange: (value: string) => void;
  error?: string;
}) {
  return (
    <div class="field">
      <label class="field-label" for="service-input">
        Service name
      </label>
      <input
        id="service-input"
        class={`field-input ${error ? "field-input--error" : ""}`}
        type="text"
        value={value}
        onInput={(event) => onChange((event.target as HTMLInputElement).value)}
        placeholder="deadfish-demo"
      />
      {error ? <div class="field-error">{error}</div> : null}
    </div>
  );
}
