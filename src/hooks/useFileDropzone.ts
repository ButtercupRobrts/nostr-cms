import { useState, useEffect, useRef } from 'react';

interface UseFileDropzoneOptions {
  onFiles: (files: FileList) => void;
  disabled?: boolean;
}

/**
 * Drag & drop state machine for file dropzones. Returns `isDragging` for
 * styling plus the four `dropzoneProps` handlers to spread on the drop target.
 * Uses a counter (not a boolean) for dragenter/leave because children of the
 * dropzone fire their own enter/leave events — a naive toggle flickers.
 */
export function useFileDropzone({ onFiles, disabled }: UseFileDropzoneOptions) {
  const [isDragging, setIsDragging] = useState(false);
  const dragCounter = useRef(0);

  const handleDragEnter = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    dragCounter.current++;
    if (e.dataTransfer.types.includes('Files')) setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    dragCounter.current--;
    if (dragCounter.current <= 0) {
      dragCounter.current = 0;
      setIsDragging(false);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    // Required to allow drop
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    dragCounter.current = 0;
    setIsDragging(false);
    if (disabled) return;
    const files = e.dataTransfer.files;
    if (files && files.length > 0) {
      onFiles(files);
    }
  };

  // Reset drag state if the user drags out of the window without dropping
  useEffect(() => {
    const reset = () => {
      dragCounter.current = 0;
      setIsDragging(false);
    };
    window.addEventListener('dragend', reset);
    window.addEventListener('drop', reset);
    return () => {
      window.removeEventListener('dragend', reset);
      window.removeEventListener('drop', reset);
    };
  }, []);

  return {
    isDragging,
    dropzoneProps: {
      onDragEnter: handleDragEnter,
      onDragLeave: handleDragLeave,
      onDragOver: handleDragOver,
      onDrop: handleDrop,
    },
  };
}
