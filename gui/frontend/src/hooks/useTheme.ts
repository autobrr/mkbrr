import { useState, useEffect, useCallback } from 'react';

export type Mode = 'light' | 'dark' | 'system';
export type ResolvedMode = 'light' | 'dark';
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

function getSystemMode(): ResolvedMode {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

export function useTheme() {
  const [state, setState] = useState<ThemeState>(() => {
    const storedMode = localStorage.getItem('mode') as Mode | null;
    const storedTheme = localStorage.getItem('theme') as ThemeName | null;

    // Default to 'system' if no preference stored
    const mode = storedMode || 'system';
    const theme = storedTheme || 'autobrr';

    return { mode, theme };
  });

  const [systemMode, setSystemMode] = useState<ResolvedMode>(getSystemMode);

  // Listen for OS theme changes
  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

    const handleChange = (e: MediaQueryListEvent) => {
      setSystemMode(e.matches ? 'dark' : 'light');
    };

    mediaQuery.addEventListener('change', handleChange);
    return () => mediaQuery.removeEventListener('change', handleChange);
  }, []);

  // Resolve the actual mode (system -> light/dark)
  const resolvedMode: ResolvedMode = state.mode === 'system' ? systemMode : state.mode;

  // Apply resolved mode (dark class on html element)
  useEffect(() => {
    const root = document.documentElement;
    if (resolvedMode === 'dark') {
      root.classList.add('dark');
    } else {
      root.classList.remove('dark');
    }
  }, [resolvedMode]);

  // Persist mode preference to localStorage
  useEffect(() => {
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
    setState(prev => {
      // Cycle through: system -> light -> dark -> system
      const nextMode: Mode = prev.mode === 'system' ? 'light' : prev.mode === 'light' ? 'dark' : 'system';
      return { ...prev, mode: nextMode };
    });
  }, []);

  return {
    mode: state.mode,
    resolvedMode,
    theme: state.theme,
    setMode,
    setTheme,
    toggleMode,
    availableThemes: AVAILABLE_THEMES,
  };
}
