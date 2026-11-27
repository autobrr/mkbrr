/*
 * Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export function OriginHint({ origin, presetName }: { origin: string | undefined; presetName: string }) {
  if (origin === 'preset') return <p className="text-xs text-muted-foreground">From preset {presetName}</p>;
  if (origin === 'tracker') return <p className="text-xs text-muted-foreground">Tracker default</p>;
  return null;
}
