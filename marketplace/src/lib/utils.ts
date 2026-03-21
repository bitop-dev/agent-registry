import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";
import type { Snippet } from "svelte";
import type { HTMLAttributes } from "svelte/elements";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// Utility types for shadcn-svelte components
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithElementRef<T extends Record<string, any> = HTMLAttributes<HTMLElement>> = T & {
  ref?: HTMLElement | null;
};

type AnyRecord = Record<string, unknown>;

export type WithoutChildren<T extends AnyRecord = AnyRecord> = Omit<T, "children">;

export type WithoutChild<T extends AnyRecord = AnyRecord> = Omit<T, "child">;

export type WithoutChildrenOrChild<T extends AnyRecord = AnyRecord> = Omit<T, "children" | "child">;
