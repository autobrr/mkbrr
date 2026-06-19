import { useEffect, useRef, useState } from 'react';
import { OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime';

/**
 * Hook that registers Wails' native file drop handler and tracks drag state.
 *
 * @param onDrop - Callback invoked with the dropped file/folder paths.
 * @returns { isDragging } - True while files are being dragged over the window.
 */
export function useFileDrop(onDrop: (paths: string[]) => void): { isDragging: boolean } {
  const [isDragging, setIsDragging] = useState(false);

  // Keep a stable ref so the effect closure never becomes stale.
  const onDropRef = useRef(onDrop);
  useEffect(() => {
    onDropRef.current = onDrop;
  });

  useEffect(() => {
    let counter = 0;

    // Force the overlay off regardless of the nesting counter.
    const reset = () => {
      counter = 0;
      setIsDragging(false);
    };

    const handleDragEnter = (e: DragEvent) => {
      if (!e.dataTransfer?.types.includes('Files')) return;
      e.preventDefault();
      counter++;
      if (counter === 1) setIsDragging(true);
    };

    const handleDragLeave = (e: DragEvent) => {
      // A dragleave with no related target, or with coordinates outside the
      // viewport, means the cursor actually left the window. Force-reset so the
      // overlay can't get stuck if a final balanced dragleave is never delivered
      // (a known cross-platform webview quirk). Otherwise just unwind the counter
      // for normal transitions between nested elements.
      if (
        !e.relatedTarget ||
        e.clientX <= 0 ||
        e.clientY <= 0 ||
        e.clientX >= window.innerWidth ||
        e.clientY >= window.innerHeight
      ) {
        reset();
        return;
      }
      counter = Math.max(0, counter - 1);
      if (counter === 0) setIsDragging(false);
    };

    const handleDragOver = (e: DragEvent) => {
      if (e.dataTransfer?.types.includes('Files')) {
        e.preventDefault();
      }
    };

    // Mouse events only resume once no drag is in progress, so a plain
    // mousemove while the overlay is up means a drag was cancelled (e.g. Esc)
    // without firing drop/dragleave — clear the stranded overlay.
    const handleMouseMove = () => {
      if (counter !== 0) reset();
    };

    window.addEventListener('dragenter', handleDragEnter);
    window.addEventListener('dragleave', handleDragLeave);
    window.addEventListener('dragover', handleDragOver);
    // Reset on native drop (Wails intercepts the actual data via OnFileDrop),
    // on a cancelled drag, and as a fallback when the pointer returns.
    window.addEventListener('drop', reset);
    window.addEventListener('dragend', reset);
    window.addEventListener('mousemove', handleMouseMove);

    // Wails native file drop – gives us real filesystem paths.
    OnFileDrop((_x, _y, paths) => {
      reset();
      if (paths && paths.length > 0) {
        onDropRef.current(paths);
      }
    }, false);

    return () => {
      window.removeEventListener('dragenter', handleDragEnter);
      window.removeEventListener('dragleave', handleDragLeave);
      window.removeEventListener('dragover', handleDragOver);
      window.removeEventListener('drop', reset);
      window.removeEventListener('dragend', reset);
      window.removeEventListener('mousemove', handleMouseMove);
      OnFileDropOff();
    };
  }, []); // Intentionally empty – setup/teardown once per mount.

  return { isDragging };
}
