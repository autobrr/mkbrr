import { Upload } from 'lucide-react';

interface DropOverlayProps {
  visible: boolean;
  label?: string;
}

/**
 * Full-screen overlay displayed while the user drags files over the window.
 * Uses fixed positioning (z-50) to sit above all page content.
 */
export function DropOverlay({ visible, label = 'Drop files here' }: DropOverlayProps) {
  if (!visible) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm pointer-events-none">
      <div className="flex flex-col items-center gap-4 px-16 py-12 rounded-2xl border-2 border-dashed border-primary/60 bg-card/90 shadow-2xl">
        <Upload className="h-14 w-14 text-primary/70" />
        <p className="text-lg font-semibold text-foreground/80">{label}</p>
      </div>
    </div>
  );
}
