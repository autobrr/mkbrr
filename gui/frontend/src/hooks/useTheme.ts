import { useState, useEffect, useCallback } from 'react';

export type Mode = 'light' | 'dark';
export type ThemeName = 'default' | 'autobrr' | 'nightwalker' | 'swizzin' | 'amethyst-haze';

export interface ThemeInfo {
  name: ThemeName;
  displayName: string;
  description: string;
}

export const AVAILABLE_THEMES: ThemeInfo[] = [
  { name: 'default', displayName: 'Default', description: 'Clean neutral theme with subtle grays' },
  { name: 'autobrr', displayName: 'Autobrr', description: 'Clean theme inspired by the autobrr project' },
  { name: 'nightwalker', displayName: 'Nightwalker', description: 'Dark theme inspired by the nightwalker project' },
  { name: 'swizzin', displayName: 'Swizzin', description: 'Light theme inspired by the swizzin project' },
  { name: 'amethyst-haze', displayName: 'Amethyst Haze', description: 'Premium purple theme' },
];

interface ThemeState {
  mode: Mode;
  theme: ThemeName;
}

export function useTheme() {
  const [state, setState] = useState<ThemeState>(() => {
    // Check localStorage first
    const storedMode = localStorage.getItem('mode') as Mode | null;
    const storedTheme = localStorage.getItem('theme') as ThemeName | null;

    const mode = storedMode || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    const theme = storedTheme || 'autobrr';

    return { mode, theme };
  });

  // Apply mode (dark class on html element)
  useEffect(() => {
    const root = document.documentElement;
    if (state.mode === 'dark') {
      root.classList.add('dark');
    } else {
      root.classList.remove('dark');
    }
    localStorage.setItem('mode', state.mode);
  }, [state.mode]);

  // Apply theme (data-theme attribute on html element)
  useEffect(() => {
    const root = document.documentElement;
    root.setAttribute('data-theme', state.theme);
    localStorage.setItem('theme', state.theme);
  }, [state.theme]);

  const setMode = useCallback((mode: Mode) => {
    setState(prev => ({ ...prev, mode }));
  }, []);

  const setTheme = useCallback((theme: ThemeName) => {
    setState(prev => ({ ...prev, theme }));
  }, []);

  const toggleMode = useCallback(() => {
    setState(prev => ({ ...prev, mode: prev.mode === 'dark' ? 'light' : 'dark' }));
  }, []);

  return {
    mode: state.mode,
    theme: state.theme,
    setMode,
    setTheme,
    toggleMode,
    availableThemes: AVAILABLE_THEMES,
  };
}
