import type { ComponentChildren } from "preact";

function stringifyValue(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch (error) {
    return String(value);
  }
}

export function KeyValueTable({
  title,
  entries,
  empty,
}: {
  title?: ComponentChildren;
  entries: Record<string, unknown>;
  empty?: string;
}) {
  const rows = Object.entries(entries ?? {}).sort(([a], [b]) => a.localeCompare(b));

  return (
    <section class="kv-section">
      {title ? <h4>{title}</h4> : null}
      {rows.length === 0 ? (
        <div class="empty-state">{empty ?? "None"}</div>
      ) : (
        <table class="kv-table">
          <tbody>
            {rows.map(([key, value]) => (
              <tr key={key}>
                <td class="kv-key">{key}</td>
                <td class="kv-value">{stringifyValue(value)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}
