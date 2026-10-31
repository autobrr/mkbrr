import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Piece length exponents (2^exp bytes)
export const PIECE_LENGTHS = [
  { value: 0, label: 'Auto' },
  { value: 14, label: '16 KiB' },
  { value: 15, label: '32 KiB' },
  { value: 16, label: '64 KiB' },
  { value: 17, label: '128 KiB' },
  { value: 18, label: '256 KiB' },
  { value: 19, label: '512 KiB' },
  { value: 20, label: '1 MiB' },
  { value: 21, label: '2 MiB' },
  { value: 22, label: '4 MiB' },
  { value: 23, label: '8 MiB' },
  { value: 24, label: '16 MiB' },
  { value: 25, label: '32 MiB' },
  { value: 26, label: '64 MiB' },
  { value: 27, label: '128 MiB' },
];
