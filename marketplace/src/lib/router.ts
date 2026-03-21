import { writable, derived } from "svelte/store";

export const hash = writable(window.location.hash.slice(1) || "/");
window.addEventListener("hashchange", () => {
  hash.set(window.location.hash.slice(1) || "/");
});

export function navigate(path: string) {
  window.location.hash = path;
}

export const currentPath = derived(hash, ($h) => $h.split("?")[0]);
export const queryParams = derived(hash, ($h) => {
  const q = $h.split("?")[1];
  return q ? new URLSearchParams(q) : new URLSearchParams();
});
