import { useEffect } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Layout } from '@/components/layout/Layout';
import { CreatePage } from '@/pages/Create';
import { InspectPage } from '@/pages/Inspect';
import { CheckPage } from '@/pages/Check';
import { ModifyPage } from '@/pages/Modify';
import { SettingsPage } from '@/pages/Settings';
import { Toaster } from '@/components/ui/sonner';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { WindowGetSize, WindowSetSize } from '../wailsjs/runtime/runtime';

const WINDOW_SIZE_KEY = 'mkbrr-window-size';
const MIN_WIDTH = 900;
const MIN_HEIGHT = 600;

function App() {
  // Restore saved window size on startup, and persist it whenever the window is resized.
  useEffect(() => {
    const saved = localStorage.getItem(WINDOW_SIZE_KEY);
    if (saved) {
      try {
        const { w, h } = JSON.parse(saved);
        if (w >= MIN_WIDTH && h >= MIN_HEIGHT) {
          WindowSetSize(w, h);
        }
      } catch {
        // Ignore malformed data.
      }
    }

    let saveTimer: ReturnType<typeof setTimeout>;
    const handleResize = () => {
      clearTimeout(saveTimer);
      saveTimer = setTimeout(async () => {
        try {
          const size = await WindowGetSize();
          localStorage.setItem(WINDOW_SIZE_KEY, JSON.stringify(size));
        } catch {
          // Ignore – non-critical.
        }
      }, 300);
    };

    window.addEventListener('resize', handleResize);
    return () => {
      window.removeEventListener('resize', handleResize);
      clearTimeout(saveTimer);
    };
  }, []);

  return (
    <ErrorBoundary>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<CreatePage />} />
            <Route path="/inspect" element={<InspectPage />} />
            <Route path="/check" element={<CheckPage />} />
            <Route path="/modify" element={<ModifyPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
        </Routes>
        <Toaster />
      </BrowserRouter>
    </ErrorBoundary>
  );
}

export default App;
