import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatPieceSize(exp: number): string {
  if (exp === 0) return '';
  const size = Math.pow(2, exp);
  if (size >= 1024 * 1024) return `${size / (1024 * 1024)} MiB`;
  if (size >= 1024) return `${size / 1024} KiB`;
  return `${size} B`;
}

export function pieceLengthOptions(range: { min: number; max: number } | null | undefined) {
  const options: { value: number; label: string }[] = [];
  if (!range) return options;
  for (let exp = range.min; exp <= range.max; exp++) {
    options.push({ value: exp, label: formatPieceSize(exp) });
  }
  return options;
}
