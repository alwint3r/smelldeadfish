const rawBase = import.meta.env.BASE_URL ?? "/";
const normalizedBase = rawBase.endsWith("/") ? rawBase.slice(0, -1) : rawBase;
const basePrefix = normalizedBase === "" ? "" : normalizedBase;

export const basePath = basePrefix;
export const basePathWithSlash = basePrefix === "" ? "/" : `${basePrefix}/`;

export function withBase(path: string): string {
  if (!path.startsWith("/")) {
    return path;
  }
  if (basePrefix === "") {
    return path;
  }
  if (path === "/") {
    return `${basePrefix}/`;
  }
  if (path.startsWith(basePrefix + "/")) {
    return path;
  }
  return `${basePrefix}${path}`;
}
