import { Component, type ErrorInfo, type ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';

// Keys mkbrr persists in localStorage. Stale or malformed values here are the
// most common cause of a render crash on mount, so the fallback offers to clear
// them. Keep in sync with the STORAGE_KEY constants in the page components.
const PERSISTED_KEYS = [
  'mkbrr-create-form',
  'mkbrr-create-result',
  'mkbrr-modify-form',
  'mkbrr-check-form',
  'mkbrr-inspect-state',
  'mkbrr-default-settings',
];

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * Catches render-phase errors anywhere in the wrapped tree and shows a
 * recoverable fallback instead of unmounting React into a blank window.
 *
 * This is a safety net for the whole class of "blank screen" bugs: a single
 * page throwing (e.g. a backend value that marshalled to null, or stale
 * persisted state) no longer takes down the entire app. It does not catch
 * errors in async code or event handlers — those are surfaced via toasts.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Render error caught by ErrorBoundary:', error, info);
  }

  handleTryAgain = () => {
    this.setState({ error: null });
  };

  handleResetSavedData = () => {
    try {
      PERSISTED_KEYS.forEach((key) => localStorage.removeItem(key));
    } catch (e) {
      console.error('Failed to clear saved data:', e);
    }
    window.location.reload();
  };

  render() {
    const { error } = this.state;
    if (!error) {
      return this.props.children;
    }

    return (
      <div className="flex h-full flex-col items-center justify-center gap-4 p-8 text-center">
        <AlertTriangle className="h-10 w-10 text-amber-500" />
        <div className="space-y-1">
          <h2 className="text-lg font-semibold">Something went wrong</h2>
          <p className="max-w-md break-words text-sm text-muted-foreground">
            {error.message || String(error)}
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={this.handleTryAgain}
            className="rounded-md border px-4 py-2 text-sm transition-colors hover:bg-muted"
          >
            Try again
          </button>
          <button
            onClick={this.handleResetSavedData}
            className="rounded-md border border-destructive/50 px-4 py-2 text-sm text-destructive transition-colors hover:bg-destructive/10"
          >
            Reset saved data &amp; reload
          </button>
        </div>
      </div>
    );
  }
}
