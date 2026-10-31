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
import { SelectPath, SelectFile, CreateTorrent, ListPresets, GetPreset, GetTrackerInfo, PlanCreate, InspectTorrent } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { getEffectiveWorkers } from './Settings';
import { useFileDrop } from '@/hooks/useFileDrop';
import { DropOverlay } from '@/components/ui/drop-overlay';

import { main, preset as presetTypes, torrent } from '../../wailsjs/go/models';

// Re-export types from generated models
type TorrentResultType = main.TorrentResult;
type PresetOptions = presetTypes.Options;
type TrackerInfoType = main.TrackerInfo;
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

// Form state that persists across navigation
interface CreateFormState {
  path: string;
  name: string;
  trackers: string[];
  webSeeds: string[];
  isPrivate: boolean;
  comment: string;
  source: string;
  pieceLengthExp: number;
  maxPieceLengthExp: number;
  targetPieceCount: number;
  excludePatterns: string;
  includePatterns: string;
  outputDir: string;
  noDate: boolean;
  noCreator: boolean;
  entropy: boolean;
  skipPrefix: boolean;
  presetName: string;
  failOnSeasonWarning: boolean;
}

const STORAGE_KEY = 'mkbrr-create-form';
const RESULT_STORAGE_KEY = 'mkbrr-create-result';

// Track if we've shown localStorage warnings to avoid spamming
let localStorageWarningShown = false;

function loadFormState(): Partial<CreateFormState> {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    return saved ? JSON.parse(saved) : {};
  } catch (e) {
    console.error('Failed to load form state from localStorage:', e);
    return {};
  }
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

function formatPieceSize(exp: number): string {
  if (exp === 0) return '';
  const size = Math.pow(2, exp);
  if (size >= 1024 * 1024) return `${size / (1024 * 1024)} MiB`;
  if (size >= 1024) return `${size / 1024} KiB`;
  return `${size} B`;
}

// Piece length exponents (2^exp bytes)
const PIECE_LENGTHS = [
  { value: 0, label: 'Auto' },
  { value: 14, label: '16 KiB' },
  { value: 15, label: '32 KiB' },
  { value: 16, label: '64 KiB' },
  { value: 17, label: '128 KiB' },
  { value: 18, label: '256 KiB' },
  { value: 19, label: '512 KiB' },
  { value: 20, label: '1 MiB' },
  { value: 21, label: '2 MiB' },
  { value: 22, label: '4 MiB' },
  { value: 23, label: '8 MiB' },
  { value: 24, label: '16 MiB' },
  { value: 25, label: '32 MiB' },
  { value: 26, label: '64 MiB' },
  { value: 27, label: '128 MiB' },
];

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

// nonBlank drops the empty inputs of a URL list.
const nonBlank = (urls: string[]) => urls.filter(u => u.trim() !== '');

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
  // Load saved form state from localStorage
  const savedState = loadFormState();

  const [path, setPath] = useState(savedState.path ?? '');
  const [name, setName] = useState(savedState.name ?? '');
  const [trackers, setTrackers] = useState<string[]>(
    Array.isArray(savedState.trackers) && savedState.trackers.length > 0 ? savedState.trackers : ['']
  );
  const [webSeeds, setWebSeeds] = useState<string[]>(
    Array.isArray(savedState.webSeeds) && savedState.webSeeds.length > 0 ? savedState.webSeeds : ['']
  );
  const [isPrivate, setIsPrivate] = useState(savedState.isPrivate ?? true);
  const [comment, setComment] = useState(savedState.comment ?? '');
  const [source, setSource] = useState(savedState.source ?? '');
  const [pieceLengthExp, setPieceLengthExp] = useState(savedState.pieceLengthExp ?? 0);
  const [maxPieceLengthExp, setMaxPieceLengthExp] = useState(savedState.maxPieceLengthExp ?? 0);
  const [targetPieceCount, setTargetPieceCount] = useState(savedState.targetPieceCount ?? 0);
  const [excludePatterns, setExcludePatterns] = useState(savedState.excludePatterns ?? '');
  const [includePatterns, setIncludePatterns] = useState(savedState.includePatterns ?? '');
  const [outputDir, setOutputDir] = useState(savedState.outputDir ?? '');
  const [noDate, setNoDate] = useState(savedState.noDate ?? false);
  const [noCreator, setNoCreator] = useState(savedState.noCreator ?? false);
  const [entropy, setEntropy] = useState(savedState.entropy ?? false);
  const [skipPrefix, setSkipPrefix] = useState(savedState.skipPrefix ?? false);
  const [presetName, setPresetName] = useState(savedState.presetName ?? '');
  const [failOnSeasonWarning, setFailOnSeasonWarning] = useState(savedState.failOnSeasonWarning ?? false);

  const [presets, setPresets] = useState<string[]>([]);
  const [isCreating, setIsCreating] = useState(false);
  const [progress, setProgress] = useState<ProgressEvent | null>(null);
  const [result, setResult] = useState<TorrentResultType | null>(null);
  const [error, setError] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [trackerInfo, setTrackerInfo] = useState<TrackerInfoType | null>(null);
  const [defaultSource, setDefaultSource] = useState('');
  const [plan, setPlan] = useState<CreatePlanType | null>(null);
  const [planError, setPlanError] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);

  // Drag-and-drop: accept any file or folder and use it as the source path.
  const { isDragging } = useFileDrop((paths) => {
    setPath(paths[0]);
    toast.success('Path set from dropped item');
  });

  useEffect(() => {
    ListPresets().then((names) => setPresets(names ?? [])).catch((e) => toast.error('Failed to load presets: ' + String(e)));
  }, []);

  // Save form state to localStorage whenever values change
  useEffect(() => {
    saveFormState({
      path,
      name,
      trackers,
      webSeeds,
      isPrivate,
      comment,
      source,
      pieceLengthExp,
      maxPieceLengthExp,
      targetPieceCount,
      excludePatterns,
      includePatterns,
      outputDir,
      noDate,
      noCreator,
      entropy,
      skipPrefix,
      presetName,
      failOnSeasonWarning,
    });
  }, [path, name, trackers, webSeeds, isPrivate, comment, source, pieceLengthExp, maxPieceLengthExp, targetPieceCount,
    excludePatterns, includePatterns, outputDir, noDate, noCreator, entropy, skipPrefix, presetName, failOnSeasonWarning]);

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

  // Look up the rules of the first tracker. Create applies only the rules of the first tracker.
  useEffect(() => {
    const firstTracker = trackers.find(t => t.trim() !== '');
    if (!firstTracker) {
      setTrackerInfo(null);
      setDefaultSource('');
      return;
    }

    const debounce = setTimeout(async () => {
      try {
        const info = await GetTrackerInfo(firstTracker);
        setDefaultSource(info?.defaultSource ?? '');
        setTrackerInfo(info?.hasCustomRules ? info : null);
      } catch (e) {
        setDefaultSource('');
        setTrackerInfo(null);
        toast.error('Failed to get tracker info: ' + String(e));
      }
    }, 300);
    return () => clearTimeout(debounce);
  }, [trackers]);

  // The source field is an override, so put the tracker default source into it.
  // Replace the field only when it is empty or still holds the previous tracker default.
  const lastDefaultSource = useRef('');
  useEffect(() => {
    const previous = lastDefaultSource.current;
    lastDefaultSource.current = defaultSource;
    setSource(current => (current === '' || current === previous ? defaultSource : current));
  }, [defaultSource]);

  // The piece length fields, the output directory, the web seeds, and the patterns are
  // left out when they are not set, so the preset fills them, as with the CLI flags.
  // The preset patterns and the form patterns both apply.
  const buildRequest = () => main.CreateRequest.createFrom({
    path,
    name,
    trackerUrls: nonBlank(trackers),
    webSeeds: nonBlank(webSeeds).length > 0 ? nonBlank(webSeeds) : undefined,
    isPrivate,
    comment,
    source,
    pieceLengthExp: pieceLengthExp || undefined,
    maxPieceLength: maxPieceLengthExp || undefined,
    targetPieceCount: targetPieceCount || undefined,
    excludePatterns: patternList(excludePatterns),
    includePatterns: patternList(includePatterns),
    outputDir: outputDir || undefined,
    noDate,
    noCreator,
    entropy,
    skipPrefix,
    failOnSeasonWarning,
    presetName,
    // Workers from settings. The backend gives a preset value priority over it.
    defaultWorkers: getEffectiveWorkers(),
  });

  // Ask for a new create plan after each change to the request. The name, the comment,
  // the source, and the toggles change the .torrent size, so they can change the piece length.
  const planRequest = buildRequest();
  const planKey = JSON.stringify(planRequest);
  useEffect(() => {
    if (!path) {
      setPlan(null);
      setPlanError('');
      return;
    }

    // A slow plan for an old path must not replace the plan for the current form.
    let stale = false;
    const debounce = setTimeout(async () => {
      try {
        const next = await PlanCreate(planRequest);
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
  }, [planKey]);

  // Reset the piece length and the max piece length if they exceed the tracker's max
  useEffect(() => {
    const trackerMax = trackerInfo?.maxPieceLength;
    if (trackerMax && pieceLengthExp > trackerMax) {
      setPieceLengthExp(0); // Reset to Auto
    }
    if (trackerMax && maxPieceLengthExp > trackerMax) {
      setMaxPieceLengthExp(0); // Reset to no limit
    }
  }, [trackerInfo?.maxPieceLength, pieceLengthExp, maxPieceLengthExp]);

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
        setOutputDir(selected);
      }
    } catch (e) {
      setError(String(e));
    }
  };

  const handleClearFields = () => {
    setPath('');
    setName('');
    setTrackers(['']);
    setWebSeeds(['']);
    setIsPrivate(true);
    setComment('');
    setSource('');
    setPieceLengthExp(0);
    setMaxPieceLengthExp(0);
    setTargetPieceCount(0);
    setExcludePatterns('');
    setIncludePatterns('');
    setOutputDir('');
    // Don't reset noDate, noCreator, entropy, skipPrefix, failOnSeasonWarning - they're preferences, not form fields
    setPresetName('');
    setResult(null);
    setError('');
    setTrackerInfo(null);
    setPlan(null);
    setPlanError('');
    clearFormState(); // Also clear localStorage
  };

  const handlePresetChange = async (value: string) => {
    if (value === 'none') {
      // Clear preset-related fields when "None" is selected
      setPresetName('');
      setTrackers(['']);
      setWebSeeds(['']);
      setSource('');
      setComment('');
      setIsPrivate(true);
      return;
    }

    setPresetName(value);
    if (value) {
      try {
        const preset = await GetPreset(value) as PresetOptions;
        if (preset) {
          // Support both camelCase (new) and PascalCase (legacy) field names
          const trackerList = preset.trackers || (preset as any).Trackers;
          const sourceVal = preset.source || (preset as any).Source;
          const privateVal = preset.private ?? (preset as any).Private;
          const commentVal = preset.comment || (preset as any).Comment;

          if (trackerList && trackerList.length > 0) setTrackers(trackerList);
          setWebSeeds(preset.webSeeds?.length ? preset.webSeeds : ['']);
          if (sourceVal) setSource(sourceVal);
          if (privateVal !== undefined) setIsPrivate(privateVal);
          if (commentVal) setComment(commentVal);

          // Preset overrides preference-like options if specified
          if (preset.noDate !== undefined) setNoDate(preset.noDate);
          if (preset.noCreator !== undefined) setNoCreator(preset.noCreator);
          if (preset.entropy !== undefined) setEntropy(preset.entropy);
          if (preset.skipPrefix !== undefined) setSkipPrefix(preset.skipPrefix);
          if (preset.failOnSeasonWarning !== undefined) setFailOnSeasonWarning(preset.failOnSeasonWarning);
        }
      } catch (e) {
        toast.error('Failed to load preset: ' + String(e));
      }
    }
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
                  onCheckedChange={setIsPrivate}
                />
                <Label htmlFor="private" className="text-sm">Private</Label>
              </div>
            </div>

            {/* Trackers */}
            <div className="space-y-1.5">
              <Label>Trackers</Label>
              <UrlList
                urls={trackers}
                onChange={setTrackers}
                placeholder="https://tracker.example.com/announce"
                addLabel="Add Tracker"
              />
            </div>

            {/* Output Directory */}
            <div className="space-y-1.5">
              <Label>Output Directory</Label>
              <div className="flex gap-2">
                <div className="relative flex-1">
                  <Input
                    value={outputDir}
                    onChange={(e) => setOutputDir(e.target.value)}
                    placeholder="Same as source"
                    className={outputDir ? 'pr-8' : ''}
                  />
                  {outputDir && (
                    <button
                      type="button"
                      onClick={() => setOutputDir('')}
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
            {planError && (
              <p className="text-xs text-destructive truncate" title={planError}>{planError}</p>
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
                      onChange={(e) => setSource(e.target.value)}
                      placeholder="e.g., tracker name"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label>Comment</Label>
                    <Input
                      value={comment}
                      onChange={(e) => setComment(e.target.value)}
                      placeholder="Optional comment"
                    />
                  </div>
                </div>

                {/* Web Seeds */}
                <div className="space-y-1.5">
                  <Label>Web Seeds</Label>
                  <UrlList
                    urls={webSeeds}
                    onChange={setWebSeeds}
                    placeholder="https://example.com/files/"
                    addLabel="Add Web Seed"
                  />
                </div>

                {/* Piece Length, Max Piece Length, Target Piece Count */}
                <div className="grid gap-4 sm:grid-cols-3">
                  {[
                    { label: 'Piece Length', value: pieceLengthExp, set: setPieceLengthExp, unset: 'Auto' },
                    { label: 'Max Piece Length', value: maxPieceLengthExp, set: setMaxPieceLengthExp, unset: 'No limit' },
                  ].map((field) => (
                    <div key={field.label} className="space-y-1.5">
                      <Label>{field.label}</Label>
                      <Select value={field.value.toString()} onValueChange={(v) => field.set(parseInt(v))}>
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {PIECE_LENGTHS
                            // If tracker has max piece length limit, filter out larger sizes
                            .filter((pl) => !(trackerInfo?.maxPieceLength && pl.value > trackerInfo.maxPieceLength))
                            .map((pl) => (
                              <SelectItem key={pl.value} value={pl.value.toString()}>
                                {pl.value === 0 ? field.unset : pl.label}
                              </SelectItem>
                            ))}
                        </SelectContent>
                      </Select>
                    </div>
                  ))}
                  <div className="space-y-1.5">
                    <Label htmlFor="targetPieceCount">Target Piece Count</Label>
                    <Input
                      id="targetPieceCount"
                      type="number"
                      min={1}
                      value={targetPieceCount || ''}
                      onChange={(e) => setTargetPieceCount(Math.max(0, parseInt(e.target.value) || 0))}
                      placeholder="Auto"
                    />
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
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="includePatterns">Include Patterns</Label>
                    <Input
                      id="includePatterns"
                      value={includePatterns}
                      onChange={(e) => setIncludePatterns(e.target.value)}
                      placeholder="e.g., *.mkv,*.mp4"
                    />
                  </div>
                </div>

                {/* Toggles */}
                <div className="flex flex-wrap gap-6">
                  <div className="flex items-center gap-2">
                    <Switch
                      id="noDate"
                      checked={noDate}
                      onCheckedChange={setNoDate}
                    />
                    <Label htmlFor="noDate" className="text-sm">Exclude date</Label>
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="noCreator"
                      checked={noCreator}
                      onCheckedChange={setNoCreator}
                    />
                    <Label htmlFor="noCreator" className="text-sm">Exclude creator</Label>
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="entropy"
                      checked={entropy}
                      onCheckedChange={setEntropy}
                    />
                    <Label htmlFor="entropy" className="text-sm">Add entropy</Label>
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="skipPrefix"
                      checked={skipPrefix}
                      onCheckedChange={setSkipPrefix}
                    />
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Label htmlFor="skipPrefix" className="text-sm cursor-help">Skip filename prefix</Label>
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">Do not add the tracker domain to the start of the output filename</p>
                      </TooltipContent>
                    </Tooltip>
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="failOnSeasonWarning"
                      checked={failOnSeasonWarning}
                      onCheckedChange={setFailOnSeasonWarning}
                    />
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Label htmlFor="failOnSeasonWarning" className="text-sm cursor-help">Fail on season warning</Label>
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">Fail if an incomplete season pack is detected (missing episodes)</p>
                      </TooltipContent>
                    </Tooltip>
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
        <Button onClick={handleCreate} disabled={isCreating || !path}>
          {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {isCreating ? 'Creating...' : 'Create Torrent'}
        </Button>
      </div>
    </div>
  );
}
