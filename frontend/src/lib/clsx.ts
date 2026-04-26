// Tiny `clsx`-style helper to avoid an extra dependency.
export function clsx(...parts: Array<string | false | null | undefined>): string {
  let out = '';
  for (const part of parts) {
    if (!part) continue;
    if (out) out += ' ';
    out += part;
  }
  return out;
}
