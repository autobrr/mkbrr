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

    const handleDragEnter = (e: DragEvent) => {
      if (!e.dataTransfer?.types.includes('Files')) return;
      e.preventDefault();
      counter++;
      if (counter === 1) setIsDragging(true);
    };

    const handleDragLeave = () => {
      counter = Math.max(0, counter - 1);
      if (counter === 0) setIsDragging(false);
    };

    const handleDragOver = (e: DragEvent) => {
      if (e.dataTransfer?.types.includes('Files')) {
        e.preventDefault();
      }
    };

    // Reset on native drop (Wails intercepts the actual data via OnFileDrop).
    const handleDrop = () => {
      counter = 0;
      setIsDragging(false);
    };

    window.addEventListener('dragenter', handleDragEnter);
    window.addEventListener('dragleave', handleDragLeave);
    window.addEventListener('dragover', handleDragOver);
    window.addEventListener('drop', handleDrop);

    // Wails native file drop – gives us real filesystem paths.
    OnFileDrop((_x, _y, paths) => {
      counter = 0;
      setIsDragging(false);
      if (paths && paths.length > 0) {
        onDropRef.current(paths);
      }
    }, false);

    return () => {
      window.removeEventListener('dragenter', handleDragEnter);
      window.removeEventListener('dragleave', handleDragLeave);
      window.removeEventListener('dragover', handleDragOver);
      window.removeEventListener('drop', handleDrop);
      OnFileDropOff();
    };
  }, []); // Intentionally empty – setup/teardown once per mount.

  return { isDragging };
}
