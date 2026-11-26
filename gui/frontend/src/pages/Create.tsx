/*
 * Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Progress } from '@/components/ui/progress';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { FolderOpen, File, Plus, X, Loader2, ChevronDown, Sparkles, FileSearch, AlertTriangle } from 'lucide-react';
import { SelectPath, SelectFile, CreateTorrent, ListPresets, ResolveCreate, PlanCreate, InspectTorrent } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { getEffectiveWorkers } from './Settings';
import { useFileDrop } from '@/hooks/useFileDrop';
import { DropOverlay } from '@/components/ui/drop-overlay';
import { formatPieceSize, pieceLengthOptions } from '@/lib/utils';

import { main, torrent } from '../../wailsjs/go/models';

// Re-export types from generated models
type TorrentResultType = main.TorrentResult;
type CreateResolutionType = main.CreateResolution;
type PieceLengthRangeType = main.PieceLengthRange;
type CreatePlanType = torrent.CreatePlan;

interface ProgressEvent {
  completed: number;
  total: number;
  hashRate: number;
  percent: number;
}

function formatHashRate(mibPerSec: number): string {
  if (mibPerSec >= 1024) {
    return `${(mibPerSec / 1024).toFixed(2)} GiB/s`;
  }
  return `${mibPerSec.toFixed(2)} MiB/s`;
}

type Overrides = Partial<Omit<torrent.CreateSettings, 'excludePatterns' | 'includePatterns' | 'workers'>>;

const presetBacked = ['trackerUrls', 'webSeeds', 'comment', 'source', 'isPrivate', 'noDate', 'noCreator', 'entropy', 'skipPrefix', 'failOnSeasonWarning'] as const;

// Form state that persists across navigation
interface CreateFormState {
  path: string;
  name: string;
  presetName: string;
  excludePatterns: string;
  includePatterns: string;
  overrides: Overrides;
  legacy?: Overrides;
}

interface LegacyFormState {
  trackers?: string[];
  webSeeds?: string[];
  isPrivate?: boolean;
  comment?: string;
  source?: string;
  pieceLengthExp?: number;
  maxPieceLengthExp?: number;
  targetPieceCount?: number;
  outputDir?: string;
  noDate?: boolean;
  noCreator?: boolean;
  entropy?: boolean;
  skipPrefix?: boolean;
  failOnSeasonWarning?: boolean;
  typedFields?: Record<string, boolean>;
}

function overridesFromLegacy(saved: LegacyFormState): { overrides: Overrides; legacy?: Overrides } {
  const overrides: Overrides = {};
  if (saved.pieceLengthExp) overrides.pieceLengthExp = saved.pieceLengthExp;
  if (saved.maxPieceLengthExp) overrides.maxPieceLength = saved.maxPieceLengthExp;
  if (saved.targetPieceCount) overrides.targetPieceCount = saved.targetPieceCount;
  if (saved.outputDir) overrides.outputDir = saved.outputDir;
  const values: Overrides = {
    trackerUrls: nonBlank(saved.trackers ?? []),
    webSeeds: nonBlank(saved.webSeeds ?? []),
    comment: saved.comment ?? '',
    source: saved.source ?? '',
    isPrivate: saved.isPrivate ?? true,
    noDate: saved.noDate ?? false,
    noCreator: saved.noCreator ?? false,
    entropy: saved.entropy ?? false,
    skipPrefix: saved.skipPrefix ?? false,
    failOnSeasonWarning: saved.failOnSeasonWarning ?? false,
  };
  const record = saved.typedFields;
  if (!record) return { overrides, legacy: values };
  const legacyNames: Record<string, string> = { trackerUrls: 'trackers' };
  for (const field of presetBacked) {
    if (record[legacyNames[field] ?? field]) Object.assign(overrides, { [field]: values[field] });
  }
  return { overrides, legacy: undefined };
}

const sameValue = (a: unknown, b: unknown) => JSON.stringify(a ?? null) === JSON.stringify(b ?? null);

async function migrateLegacy(base: Overrides, legacy: Overrides, presetName: string): Promise<Overrides> {
  const overrides = { ...base };
  const differs = async (fields: readonly (typeof presetBacked)[number][]) => {
    const resolved = await ResolveCreate(main.CreateRequest.createFrom({ ...overrides, presetName }));
    for (const field of fields) {
      const value = legacy[field];
      const shown = resolved.settings[field] ?? (Array.isArray(value) ? [] : undefined);
      if (!sameValue(value, shown)) Object.assign(overrides, { [field]: value });
    }
  };
  await differs(['trackerUrls', 'webSeeds']);
  await differs(presetBacked.filter((f) => f !== 'trackerUrls' && f !== 'webSeeds'));
  return overrides;
}

const STORAGE_KEY = 'mkbrr-create-form';
const RESULT_STORAGE_KEY = 'mkbrr-create-result';

// Track if we've shown localStorage warnings to avoid spamming
let localStorageWarningShown = false;

function loadFormState(): CreateFormState {
  let saved: Partial<CreateFormState> & LegacyFormState = {};
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    saved = raw ? JSON.parse(raw) : {};
  } catch (e) {
    console.error('Failed to load form state from localStorage:', e);
  }
  return {
    path: saved.path ?? '',
    name: saved.name ?? '',
    presetName: saved.presetName ?? '',
    excludePatterns: saved.excludePatterns ?? '',
    includePatterns: saved.includePatterns ?? '',
    ...(saved.overrides ? { overrides: saved.overrides, legacy: saved.legacy } : overridesFromLegacy(saved)),
  };
}

function saveFormState(state: CreateFormState) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch (e) {
    console.error('Failed to save form state to localStorage:', e);
    // Show warning once per session
    if (!localStorageWarningShown) {
      localStorageWarningShown = true;
      if (e instanceof DOMException && e.name === 'QuotaExceededError') {
        toast.warning('Storage full: Form state cannot be saved. Consider clearing browser data.');
      } else {
        toast.warning('Unable to save form state for persistence.');
      }
    }
  }
}

function clearFormState() {
  try {
    localStorage.removeItem(STORAGE_KEY);
    localStorage.removeItem(RESULT_STORAGE_KEY);
  } catch (e) {
    console.error('Failed to clear form state from localStorage:', e);
  }
}

// UrlList is a list of URL inputs with add and remove buttons.
function UrlList({ urls, onChange, placeholder, addLabel }: {
  urls: string[];
  onChange: (urls: string[]) => void;
  placeholder: string;
  addLabel: string;
}) {
  return (
    <div className="space-y-2">
      {urls.map((url, index) => (
        <div key={index} className="flex gap-2">
          <Input
            value={url}
            onChange={(e) => onChange(urls.map((u, i) => (i === index ? e.target.value : u)))}
            placeholder={placeholder}
            className="flex-1"
          />
          {urls.length > 1 && (
            <Button variant="outline" size="icon" onClick={() => onChange(urls.filter((_, i) => i !== index))}>
              <X className="h-4 w-4" />
            </Button>
          )}
        </div>
      ))}
      <Button variant="outline" size="sm" onClick={() => onChange([...urls, ''])}>
        <Plus className="mr-2 h-4 w-4" />
        {addLabel}
      </Button>
    </div>
  );
}

function OriginHint({ origin, presetName }: { origin: string | undefined; presetName: string }) {
  if (origin === 'preset') return <p className="text-xs text-muted-foreground">From preset {presetName}</p>;
  if (origin === 'tracker') return <p className="text-xs text-muted-foreground">Tracker default</p>;
  return null;
}

// nonBlank drops the empty inputs of a URL list.
const nonBlank = (urls: string[]) => urls.filter(u => u.trim() !== '');
const orBlank = (urls: string[] | undefined) => (urls && urls.length > 0 ? urls : ['']);

// patternList sends a pattern field as one comma-separated group, as the CLI flag does.
// An empty field sends nothing, so only the preset patterns apply.
const patternList = (patterns: string) => (patterns.trim() ? [patterns.trim()] : undefined);

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export function CreatePage() {
  const navigate = useNavigate();
  const [savedState] = useState(loadFormState);

  const [path, setPath] = useState(savedState.path);
  const [name, setName] = useState(savedState.name);
  const [presetName, setPresetName] = useState(savedState.presetName);
  const [excludePatterns, setExcludePatterns] = useState(savedState.excludePatterns);
  const [includePatterns, setIncludePatterns] = useState(savedState.includePatterns);
  const [overrides, setOverrides] = useState<Overrides>(savedState.overrides);
  const [legacy, setLegacy] = useState(savedState.legacy);
  const override = <K extends keyof Overrides>(field: K, value: Overrides[K]) => setOverrides((o) => ({ ...o, [field]: value }));
  const clearOverride = (field: keyof Overrides) => setOverrides(({ [field]: _, ...rest }) => rest);

  const [presets, setPresets] = useState<string[]>([]);
  const [isCreating, setIsCreating] = useState(false);
  const [progress, setProgress] = useState<ProgressEvent | null>(null);
  const [result, setResult] = useState<TorrentResultType | null>(null);
  const [error, setError] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [resolved, setResolved] = useState<{ presetName: string; resolution: CreateResolutionType } | null>(null);
  const [resolveError, setResolveError] = useState('');
  const [plan, setPlan] = useState<CreatePlanType | null>(null);
  const [planError, setPlanError] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [presetFileRead, setPresetFileRead] = useState(0);

  // Drag-and-drop: accept any file or folder and use it as the source path.
  const { isDragging } = useFileDrop((paths) => {
    setPath(paths[0]);
    toast.success('Path set from dropped item');
  });

  const loadPresets = () => {
    ListPresets().then((names) => setPresets(names ?? [])).catch((e) => toast.error('Failed to load presets: ' + String(e)));
  };
  useEffect(loadPresets, []);

  // Save form state to localStorage whenever values change
  useEffect(() => {
    saveFormState({ path, name, presetName, excludePatterns, includePatterns, overrides, legacy });
  }, [path, name, presetName, excludePatterns, includePatterns, overrides, legacy]);

  useEffect(() => {
    if (!legacy) return;
    migrateLegacy(overrides, legacy, presetName)
      .catch((e) => {
        console.error('Failed to migrate the saved form state:', e);
        return { ...overrides, ...legacy };
      })
      .then((migrated) => {
        setOverrides((current) => ({ ...migrated, ...current }));
        setLegacy(undefined);
      });
  }, []);

  useEffect(() => {
    const reread = () => {
      setPresetFileRead((n) => n + 1);
      loadPresets();
    };
    window.addEventListener('focus', reread);
    return () => window.removeEventListener('focus', reread);
  }, []);

  // Check for pending result on mount (in case user navigated away during creation)
  useEffect(() => {
    try {
      const savedResult = localStorage.getItem(RESULT_STORAGE_KEY);
      if (savedResult) {
        const parsed = JSON.parse(savedResult);
        setResult(parsed);
        setDialogOpen(true);
      }
    } catch (e) {
      console.error('Failed to load saved result from localStorage:', e);
    }
  }, []);

  useEffect(() => {
    const cancel = EventsOn('create:progress', (data: ProgressEvent) => {
      setProgress(data);
    });
    return () => {
      cancel();
    };
  }, []);

  const buildRequest = () => main.CreateRequest.createFrom({
    ...overrides,
    path,
    name,
    trackerUrls: overrides.trackerUrls && nonBlank(overrides.trackerUrls),
    webSeeds: overrides.webSeeds && nonBlank(overrides.webSeeds),
    excludePatterns: patternList(excludePatterns),
    includePatterns: patternList(includePatterns),
    presetName,
    // Workers from settings. The backend gives a preset value priority over it.
    defaultWorkers: getEffectiveWorkers(),
  });
  const request = buildRequest();
  const requestKey = JSON.stringify({ request, presetFileRead });

  const resolveKey = JSON.stringify({ request: { ...request, path: '', name: '' }, presetFileRead });
  useEffect(() => {
    if (legacy) return;
    let stale = false;
    ResolveCreate(request)
      .then((next) => {
        if (stale) return;
        setResolved({ presetName, resolution: next });
        setResolveError('');
      })
      .catch((e) => {
        if (stale) return;
        setResolveError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      stale = true;
    };
  }, [resolveKey, legacy]);

  const resolution = resolved?.presetName === presetName ? resolved.resolution : null;
  const settings = resolution?.settings;
  const origins = resolution?.origins;
  const allowedPieceLengths = resolution?.allowedPieceLengths;
  const trackerInfo = resolution?.tracker ?? null;
  const hint = (field: keyof Overrides) => (
    !resolveError && overrides[field] === undefined && <OriginHint origin={origins?.[field]} presetName={presetName} />
  );
  const patternHint = (field: 'excludePatterns' | 'includePatterns') => (
    !resolveError && origins?.[field] === 'preset' && (
      <p className="text-xs text-muted-foreground">With preset {presetName}: {settings?.[field]?.join(', ')}</p>
    )
  );

  const trackers = overrides.trackerUrls ?? orBlank(settings?.trackerUrls);
  const webSeeds = overrides.webSeeds ?? orBlank(settings?.webSeeds);
  const isPrivate = overrides.isPrivate ?? settings?.isPrivate ?? false;
  const comment = overrides.comment ?? settings?.comment ?? '';
  const source = overrides.source ?? settings?.source ?? '';
  const outputDir = overrides.outputDir ?? settings?.outputDir ?? '';
  const pieceLengthExp = overrides.pieceLengthExp ?? settings?.pieceLengthExp ?? 0;
  const maxPieceLengthExp = overrides.maxPieceLength ?? settings?.maxPieceLength ?? 0;
  const targetPieceCount = overrides.targetPieceCount ?? settings?.targetPieceCount ?? 0;
  type Toggle = 'noDate' | 'noCreator' | 'entropy' | 'skipPrefix' | 'failOnSeasonWarning';
  const checked = (field: Toggle) => overrides[field] ?? settings?.[field] ?? false;

  // Ask for a new create plan after each change to the request. The name, the comment,
  // the source, and the toggles change the .torrent size, so they can change the piece length.
  useEffect(() => {
    if (legacy) return;
    if (!path) {
      setPlan(null);
      setPlanError('');
      return;
    }

    // A slow plan for an old path must not replace the plan for the current form.
    let stale = false;
    const debounce = setTimeout(async () => {
      try {
        const next = await PlanCreate(request);
        if (stale) return;
        setPlan(next);
        setPlanError('');
      } catch (e) {
        if (stale) return;
        setPlan(null);
        setPlanError(e instanceof Error ? e.message : String(e));
      }
    }, 300);
    return () => {
      stale = true;
      clearTimeout(debounce);
    };
  }, [requestKey, legacy]);

  useEffect(() => {
    if (!allowedPieceLengths) return;
    const outside = (exp: number | undefined, range: PieceLengthRangeType) => !!exp && (exp < range.min || exp > range.max);
    if (outside(overrides.pieceLengthExp, allowedPieceLengths.pieceLength)) clearOverride('pieceLengthExp');
    if (outside(overrides.maxPieceLength, allowedPieceLengths.maxPieceLength)) clearOverride('maxPieceLength');
  }, [allowedPieceLengths, overrides.pieceLengthExp, overrides.maxPieceLength]);

  const handleSelectFolder = async () => {
    try {
      const selected = await SelectPath();
      if (selected) {
        setPath(selected);
      }
    } catch (e) {
      setError(String(e));
    }
  };

  const handleSelectFile = async () => {
    try {
      const selected = await SelectFile();
      if (selected) {
        setPath(selected);
      }
    } catch (e) {
      setError(String(e));
    }
  };

  const handleSelectOutputDir = async () => {
    try {
      const selected = await SelectPath();
      if (selected) {
        override('outputDir', selected);
      }
    } catch (e) {
      setError(String(e));
    }
  };

  const handleClearFields = () => {
    setPath('');
    setName('');
    setOverrides(({ noDate, noCreator, entropy, skipPrefix, failOnSeasonWarning }) => (
      { noDate, noCreator, entropy, skipPrefix, failOnSeasonWarning }
    ));
    setExcludePatterns('');
    setIncludePatterns('');
    setPresetName('');
    setResult(null);
    setError('');
    setPlan(null);
    setPlanError('');
    clearFormState(); // Also clear localStorage
  };

  const handlePresetChange = (value: string) => {
    setPresetName(value === 'none' ? '' : value);
    const inherited = (field: string, urls: string[]) => (
      (field === 'trackerUrls' || field === 'webSeeds') && sameValue(nonBlank(urls), settings?.[field] ?? [])
    );
    setOverrides((o) => Object.fromEntries(Object.entries(o).filter(([field, v]) => (
      Array.isArray(v) ? nonBlank(v).length > 0 && !inherited(field, v) : v !== ''
    ))));
  };

  const handleDialogClose = () => {
    setDialogOpen(false);
    setResult(null);
    setError('');
    // Clear saved result from localStorage
    try {
      localStorage.removeItem(RESULT_STORAGE_KEY);
    } catch (e) {
      console.error('Failed to clear saved result from localStorage:', e);
    }
  };

  const handleInspectResult = async () => {
    if (!result?.path) return;
    try {
      const info = await InspectTorrent(result.path);
      localStorage.setItem('mkbrr-inspect-state', JSON.stringify({ torrentInfo: info }));
      handleDialogClose();
      navigate('/inspect');
    } catch (e) {
      toast.error('Failed to inspect torrent: ' + String(e));
    }
  };

  const handleCreate = async () => {
    if (!path) {
      setError('Please select a file or folder');
      return;
    }

    setError('');
    setResult(null);
    setProgress(null);
    setIsCreating(true);
    setDialogOpen(true);

    try {
      const res = await CreateTorrent(buildRequest());
      setResult(res as TorrentResultType);
      // Save result to localStorage in case user navigates away
      try {
        localStorage.setItem(RESULT_STORAGE_KEY, JSON.stringify(res));
      } catch (e) {
        console.error('Failed to save result to localStorage:', e);
      }
    } catch (e) {
      setError(String(e));
    } finally {
      setIsCreating(false);
      setProgress(null);
    }
  };

  return (
    <div className="flex flex-col h-full">
      <DropOverlay visible={isDragging} label="Drop a file or folder to set the source path" />
      <div className="flex-1 overflow-auto p-6 space-y-4">
        <div>
          <h1 className="text-2xl font-semibold">Create Torrent</h1>
          <p className="text-sm text-muted-foreground">Create a new torrent file from files or folders</p>
        </div>

        {/* Main Form Card */}
        <Card>
          <CardContent className="space-y-4">
            {/* Source Path */}
            <div className="space-y-1.5">
              <Label>Source Path</Label>
              <div className="flex gap-2">
                <div className="relative flex-1">
                  <Input
                    value={path}
                    onChange={(e) => setPath(e.target.value)}
                    placeholder="/path/to/file/or/folder"
                    className={path ? 'pr-8' : ''}
                  />
                  {path && (
                    <button
                      type="button"
                      onClick={() => { setPath(''); setResult(null); setError(''); }}
                      className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                      aria-label="Clear"
                      title="Clear"
                    >
                      <X className="h-4 w-4" />
                    </button>
                  )}
                </div>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="outline" size="icon" onClick={handleSelectFile}>
                      <File className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Select File</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="outline" size="icon" onClick={handleSelectFolder}>
                      <FolderOpen className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Select Folder</TooltipContent>
                </Tooltip>
              </div>
            </div>

            {/* Preset + Private inline */}
            <div className="flex gap-4 items-end">
              {presets.length > 0 && (
                <div className="flex-1 space-y-1.5">
                  <Label>Preset</Label>
                  <Select value={presetName} onValueChange={handlePresetChange}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select a preset" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">None</SelectItem>
                      {presets.map((p) => (
                        <SelectItem key={p} value={p}>
                          {p}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
              <div className="flex items-center gap-2 pb-2">
                <Switch
                  id="private"
                  checked={isPrivate}
                  onCheckedChange={(v) => override('isPrivate', v)}
                />
                <Label htmlFor="private" className="text-sm">Private</Label>
                {hint('isPrivate')}
              </div>
            </div>

            {/* Trackers */}
            <div className="space-y-1.5">
              <Label>Trackers</Label>
              <UrlList
                urls={trackers}
                onChange={(urls) => override('trackerUrls', urls)}
                placeholder="https://tracker.example.com/announce"
                addLabel="Add Tracker"
              />
              {hint('trackerUrls')}
            </div>

            {/* Output Directory */}
            <div className="space-y-1.5">
              <Label>Output Directory</Label>
              <div className="flex gap-2">
                <div className="relative flex-1">
                  <Input
                    value={outputDir}
                    onChange={(e) => override('outputDir', e.target.value)}
                    placeholder="Same as source"
                    className={outputDir ? 'pr-8' : ''}
                  />
                  {outputDir && (
                    <button
                      type="button"
                      onClick={() => override('outputDir', '')}
                      className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                      aria-label="Clear"
                      title="Clear"
                    >
                      <X className="h-4 w-4" />
                    </button>
                  )}
                </div>
                <Button variant="outline" size="icon" onClick={handleSelectOutputDir}>
                  <FolderOpen className="h-4 w-4" />
                </Button>
              </div>
              {hint('outputDir')}
            </div>

            {/* Create preview: the create plan, and the rules of the first tracker */}
            {(plan || trackerInfo) && (
              <div className={`flex items-start gap-3 p-3 rounded-lg border ${trackerInfo ? 'bg-emerald-500/10 border-emerald-500/20' : 'bg-muted/50'}`}>
                {trackerInfo ? (
                  <Sparkles className="h-4 w-4 text-emerald-500 mt-0.5 flex-shrink-0" />
                ) : (
                  <FileSearch className="h-4 w-4 text-muted-foreground mt-0.5 flex-shrink-0" />
                )}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-4">
                    <p className={`text-sm font-medium ${trackerInfo ? 'text-emerald-600 dark:text-emerald-400' : ''}`}>
                      {trackerInfo ? 'Tracker optimizations detected' : 'Create plan'}
                    </p>
                    {plan && (
                      <div className={`flex items-center gap-2 px-2.5 py-1 rounded-md ${trackerInfo ? 'bg-emerald-500/20' : 'bg-muted'}`}>
                        <span className="text-xs text-muted-foreground">Piece size:</span>
                        <span className="text-sm font-bold">{formatPieceSize(plan.piece_length_exponent)}</span>
                      </div>
                    )}
                  </div>
                  <div className="flex flex-wrap gap-x-4 gap-y-1 mt-1.5 text-xs text-muted-foreground">
                    {plan && (
                      <span>Content: <span className="font-medium text-foreground">{formatBytes(plan.content_size)}</span></span>
                    )}
                    {trackerInfo?.defaultSource && (
                      <span>Source: <span className="font-medium text-foreground">{trackerInfo.defaultSource}</span></span>
                    )}
                    {trackerInfo && trackerInfo.maxPieceLength > 0 && (
                      <span>Max piece: <span className="font-medium text-foreground">{formatPieceSize(trackerInfo.maxPieceLength)}</span></span>
                    )}
                    {trackerInfo && trackerInfo.maxTorrentSize > 0 && (
                      <span>Max .torrent: <span className="font-medium text-foreground">{formatBytes(trackerInfo.maxTorrentSize)}</span></span>
                    )}
                  </div>
                  {plan?.output_path && (
                    <p className="text-xs mt-1 text-muted-foreground">
                      Output: <span className="font-mono text-foreground break-all">{plan.output_path}</span>
                    </p>
                  )}
                  {plan?.notices?.map((notice, i) => (
                    <p key={i} className={`text-xs mt-1 ${notice.warn ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground'}`}>
                      {notice.text}
                    </p>
                  ))}
                  {plan?.season_pack?.is_season_pack && (
                    <p className={`flex items-center gap-1 text-xs mt-1 ${plan.season_pack.is_suspicious ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground'}`}>
                      {plan.season_pack.is_suspicious && <AlertTriangle className="h-3 w-3 flex-shrink-0" />}
                      Season {plan.season_pack.season}: {plan.season_pack.video_file_count} video files, highest episode {plan.season_pack.max_episode}
                      {plan.season_pack.missing_episodes?.length > 0 && `, missing episodes ${plan.season_pack.missing_episodes.join(', ')}`}
                    </p>
                  )}
                </div>
              </div>
            )}
            {(resolveError || planError) && (
              <p className="text-xs text-destructive truncate" title={resolveError || planError}>{resolveError || planError}</p>
            )}
          </CardContent>
        </Card>

        {/* Advanced Options - Collapsible */}
        <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
          <div className="rounded-lg border bg-card">
            <CollapsibleTrigger asChild>
              <div className="flex items-center justify-between px-4 py-2 cursor-pointer hover:bg-muted/50 transition-colors rounded-lg">
                <span className="text-sm font-medium">Advanced Options</span>
                <ChevronDown className={`h-4 w-4 text-muted-foreground transition-transform ${advancedOpen ? 'rotate-180' : ''}`} />
              </div>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <div className="px-4 pb-4 space-y-4 pt-4">
                {/* Name Override */}
                <div className="space-y-1.5">
                  <Label>Name Override</Label>
                  <Input
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Use source name"
                  />
                </div>

                {/* Source + Comment */}
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <Label>Source Tag</Label>
                    <Input
                      value={source}
                      onChange={(e) => override('source', e.target.value)}
                      placeholder="e.g., tracker name"
                    />
                    {hint('source')}
                  </div>
                  <div className="space-y-1.5">
                    <Label>Comment</Label>
                    <Input
                      value={comment}
                      onChange={(e) => override('comment', e.target.value)}
                      placeholder="Optional comment"
                    />
                    {hint('comment')}
                  </div>
                </div>

                {/* Web Seeds */}
                <div className="space-y-1.5">
                  <Label>Web Seeds</Label>
                  <UrlList
                    urls={webSeeds}
                    onChange={(urls) => override('webSeeds', urls)}
                    placeholder="https://example.com/files/"
                    addLabel="Add Web Seed"
                  />
                  {hint('webSeeds')}
                </div>

                {/* Piece Length, Max Piece Length, Target Piece Count */}
                <div className="grid gap-4 sm:grid-cols-3">
                  {[
                    { label: 'Piece Length', field: 'pieceLengthExp' as const, value: pieceLengthExp, unset: 'Auto', range: allowedPieceLengths?.pieceLength },
                    { label: 'Max Piece Length', field: 'maxPieceLength' as const, value: maxPieceLengthExp, unset: 'No limit', range: allowedPieceLengths?.maxPieceLength },
                  ].map((field) => (
                    <div key={field.label} className="space-y-1.5">
                      <Label>{field.label}</Label>
                      <Select
                        value={field.value.toString()}
                        onValueChange={(v) => (v === '0' ? clearOverride(field.field) : override(field.field, parseInt(v)))}
                      >
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="0">{field.unset}</SelectItem>
                          {pieceLengthOptions(field.range).map((pl) => (
                            <SelectItem key={pl.value} value={pl.value.toString()}>{pl.label}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      {hint(field.field)}
                    </div>
                  ))}
                  <div className="space-y-1.5">
                    <Label htmlFor="targetPieceCount">Target Piece Count</Label>
                    <Input
                      id="targetPieceCount"
                      type="number"
                      min={1}
                      value={targetPieceCount || ''}
                      onChange={(e) => {
                        const count = Math.max(0, parseInt(e.target.value) || 0);
                        if (count) override('targetPieceCount', count);
                        else clearOverride('targetPieceCount');
                      }}
                      placeholder="Auto"
                    />
                    {hint('targetPieceCount')}
                  </div>
                </div>

                {/* Exclude + Include patterns */}
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <Label htmlFor="excludePatterns">Exclude Patterns</Label>
                    <Input
                      id="excludePatterns"
                      value={excludePatterns}
                      onChange={(e) => setExcludePatterns(e.target.value)}
                      placeholder="e.g., *.nfo,*.jpg"
                    />
                    {patternHint('excludePatterns')}
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="includePatterns">Include Patterns</Label>
                    <Input
                      id="includePatterns"
                      value={includePatterns}
                      onChange={(e) => setIncludePatterns(e.target.value)}
                      placeholder="e.g., *.mkv,*.mp4"
                    />
                    {patternHint('includePatterns')}
                  </div>
                </div>

                {/* Toggles */}
                <div className="flex flex-wrap gap-6">
                  <div className="flex items-center gap-2">
                    <Switch
                      id="noDate"
                      checked={checked('noDate')}
                      onCheckedChange={(v) => override('noDate', v)}
                    />
                    <Label htmlFor="noDate" className="text-sm">Exclude date</Label>
                    {hint('noDate')}
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="noCreator"
                      checked={checked('noCreator')}
                      onCheckedChange={(v) => override('noCreator', v)}
                    />
                    <Label htmlFor="noCreator" className="text-sm">Exclude creator</Label>
                    {hint('noCreator')}
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="entropy"
                      checked={checked('entropy')}
                      onCheckedChange={(v) => override('entropy', v)}
                    />
                    <Label htmlFor="entropy" className="text-sm">Add entropy</Label>
                    {hint('entropy')}
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="skipPrefix"
                      checked={checked('skipPrefix')}
                      onCheckedChange={(v) => override('skipPrefix', v)}
                    />
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Label htmlFor="skipPrefix" className="text-sm cursor-help">Skip filename prefix</Label>
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">Do not add the tracker domain to the start of the output filename</p>
                      </TooltipContent>
                    </Tooltip>
                    {hint('skipPrefix')}
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="failOnSeasonWarning"
                      checked={checked('failOnSeasonWarning')}
                      onCheckedChange={(v) => override('failOnSeasonWarning', v)}
                    />
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Label htmlFor="failOnSeasonWarning" className="text-sm cursor-help">Fail on season warning</Label>
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">Fail if an incomplete season pack is detected (missing episodes)</p>
                      </TooltipContent>
                    </Tooltip>
                    {hint('failOnSeasonWarning')}
                  </div>
                </div>
              </div>
            </CollapsibleContent>
          </div>
        </Collapsible>

        {/* Validation Error (shown inline when dialog is closed) */}
        {!dialogOpen && error && (
          <Card className="border-destructive">
            <CardContent className="py-3">
              <p className="text-destructive text-sm">{error}</p>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Progress/Result Dialog */}
      <Dialog
        open={dialogOpen}
        onOpenChange={(open) => {
          if (!isCreating && !open) {
            handleDialogClose();
          }
        }}
      >
        <DialogContent className="sm:max-w-md" onInteractOutside={(e) => isCreating && e.preventDefault()}>
          {isCreating ? (
            <>
              <DialogHeader>
                <DialogTitle>Creating Torrent</DialogTitle>
                <DialogDescription>
                  Please wait while your torrent is being created...
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-3 py-4">
                <div className="flex justify-between text-sm">
                  <span className="font-medium">Hashing files</span>
                  <span className="text-muted-foreground">{progress?.percent.toFixed(0) ?? 0}%</span>
                </div>
                <Progress value={progress?.percent ?? 0} />
                {progress && (
                  <div className="flex justify-between text-xs text-muted-foreground">
                    <span>{progress.completed} / {progress.total} pieces</span>
                    <span>{formatHashRate(progress.hashRate)}</span>
                  </div>
                )}
              </div>
            </>
          ) : result ? (
            <>
              <DialogHeader>
                <DialogTitle className="text-emerald-600 dark:text-emerald-400">
                  Torrent Created Successfully
                </DialogTitle>
              </DialogHeader>
              {result.warning && (
                <Card className="border-amber-500 bg-amber-500/10 mb-2">
                  <CardContent className="py-2">
                    <p className="text-amber-600 dark:text-amber-400 text-sm">{result.warning}</p>
                  </CardContent>
                </Card>
              )}
              {result.seasonPackInfo?.isSuspicious && (
                <Card className="border-amber-500 bg-amber-500/10 mb-2">
                  <CardContent className="py-3">
                    <div className="flex items-start gap-2">
                      <AlertTriangle className="h-4 w-4 text-amber-500 mt-0.5 flex-shrink-0" />
                      <div className="space-y-1">
                        <p className="text-amber-600 dark:text-amber-400 text-sm font-medium">
                          Possible incomplete season pack detected
                        </p>
                        <div className="text-xs text-muted-foreground space-y-0.5">
                          <p>Season: {result.seasonPackInfo.season}</p>
                          <p>Highest episode: {result.seasonPackInfo.maxEpisode}</p>
                          <p>Video files: {result.seasonPackInfo.videoFileCount}</p>
                          {result.seasonPackInfo.missingEpisodes && result.seasonPackInfo.missingEpisodes.length > 0 && (
                            <p>Missing episodes: {result.seasonPackInfo.missingEpisodes.join(', ')}</p>
                          )}
                        </div>
                        <p className="text-xs text-amber-600 dark:text-amber-400 mt-1">
                          Check files before uploading.
                        </p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              )}
              <div className="grid grid-cols-[80px_1fr] gap-x-2 gap-y-2 text-sm py-4">
                <span className="text-muted-foreground">Path</span>
                <span className="font-mono text-xs break-all">{result.path}</span>
                <span className="text-muted-foreground">Hash</span>
                <span className="font-mono text-xs break-all">{result.infoHash}</span>
                <span className="text-muted-foreground">Size</span>
                <span>{formatBytes(result.size)}</span>
                <span className="text-muted-foreground">Pieces</span>
                <span>{result.pieceCount}</span>
                <span className="text-muted-foreground">Files</span>
                <span>{result.fileCount}</span>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={handleInspectResult}>
                  <FileSearch className="mr-2 h-4 w-4" />
                  Inspect
                </Button>
                <Button onClick={handleDialogClose}>Close</Button>
              </DialogFooter>
            </>
          ) : error ? (
            <>
              <DialogHeader>
                <DialogTitle className="text-destructive">Error</DialogTitle>
              </DialogHeader>
              <p className="text-sm py-4 break-words">{error}</p>
              <DialogFooter>
                <Button variant="outline" onClick={handleDialogClose}>Close</Button>
              </DialogFooter>
            </>
          ) : null}
        </DialogContent>
      </Dialog>

      <div className="bg-background p-4 flex justify-end gap-2">
        <Button variant="outline" onClick={handleClearFields} disabled={isCreating}>
          Reset
        </Button>
        <Button onClick={handleCreate} disabled={isCreating || !path || !!legacy}>
          {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {isCreating ? 'Creating...' : 'Create Torrent'}
        </Button>
      </div>
    </div>
  );
}
