/** Escapes HTML-sensitive characters in a value for safe inline rendering. */
export function escapeHtml(v: any) {
  const map: any = {
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  };
  return String(v).replace(/[&<>"']/g, (m) => map[m]);
}

/** Converts an arbitrary string into a lowercase slug segment. */
export function slugifySegment(v: string) {
  return String(v).toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}

/** Normalizes a branch name to characters accepted by the backend workflow. */
export function normalizeBranchName(v: string) {
  return String(v).replace(/[^a-zA-Z0-9._/-]/g, "-");
}
