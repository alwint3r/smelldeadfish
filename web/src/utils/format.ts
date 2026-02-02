export function nanoToMs(nano: number): number {
  return Math.floor(nano / 1_000_000);
}

export function formatTimestamp(nano: number): string {
  const date = new Date(nanoToMs(nano));
  return date.toLocaleString();
}

export function formatDuration(nano: number): string {
  if (!Number.isFinite(nano) || nano <= 0) {
    return "0ms";
  }
  const ms = nano / 1_000_000;
  if (ms < 1) {
    return `${(ms * 1000).toFixed(1)}us`;
  }
  if (ms < 1000) {
    return `${ms.toFixed(2)}ms`;
  }
  const s = ms / 1000;
  if (s < 60) {
    return `${s.toFixed(2)}s`;
  }
  const m = Math.floor(s / 60);
  const sRemainder = s % 60;
  return `${m}m ${sRemainder.toFixed(1)}s`;
}

export function formatCount(value: number): string {
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)}k`;
  }
  return String(value);
}
