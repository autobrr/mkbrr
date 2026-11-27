/*
 * Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { FolderOpen, Plus, X, Loader2, ChevronDown, FileSearch } from 'lucide-react';
import { SelectTorrentFile, SelectPath, ModifyTorrent, ListPresets, ResolveModify, InspectTorrent } from '../../wailsjs/go/main/App';
import { useFileDrop } from '@/hooks/useFileDrop';
import { DropOverlay } from '@/components/ui/drop-overlay';
import { OriginHint } from '@/components/ui/origin-hint';

import { main, torrent } from '../../wailsjs/go/models';

type ModifyResult = main.ModifyResult;
type ModifyResolution = torrent.ModifyResolution;

type PrivateMode = 'unchanged' | 'private' | 'public' | 'remove';
type EntropyMode = 'keep' | 'add' | 'remove';
type TrackerMode = 'keep' | 'replace' | 'remove' | 'clear';
type WebSeedMode = 'keep' | 'replace' | 'clear';

interface Picks {
  trackerMode?: Exclude<TrackerMode, 'keep'>;
  trackers?: string[];
  removeTrackers?: string[];
  webSeedMode?: Exclude<WebSeedMode, 'keep'>;
  webSeeds?: string[];
  privateMode?: Exclude<PrivateMode, 'unchanged'>;
  entropyMode?: EntropyMode;
  source?: string;
  comment?: string;
  outputDir?: string;
  noDate?: true;
  noCreator?: true;
  skipPrefix?: true;
}

interface ModifyFormState {
  torrentPath: string;
  presetName: string;
  name: string;
  outputPattern: string;
  picks: Picks;
}

interface LegacyFormState {
  outputDir?: string;
  trackerMode?: TrackerMode;
  trackers?: string[];
  removeTrackers?: string[];
  webSeedMode?: WebSeedMode;
  webSeeds?: string[];
  privateMode?: PrivateMode;
  entropyMode?: EntropyMode;
  source?: string;
  comment?: string;
  noDate?: boolean;
  noCreator?: boolean;
  skipPrefix?: boolean;
}

function RadioRow<T extends string>({ name, value, options, onChange }: {
  name: string;
  value: T;
  options: [T, string][];
  onChange: (value: T) => void;
}) {
  return (
    <div className="flex flex-wrap gap-4">
      {options.map(([option, label]) => (
        <label key={option} className="flex items-center gap-2 cursor-pointer">
          <input
            type="radio"
            name={name}
            checked={value === option}
            onChange={() => onChange(option)}
            className="w-4 h-4"
          />
          <span className="text-sm">{label}</span>
        </label>
      ))}
    </div>
  );
}

function UrlListInput({ urls, onChange, placeholder, addLabel }: {
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
            <Button
              variant="outline"
              size="icon"
              onClick={() => onChange(urls.filter((_, i) => i !== index))}
            >
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

const nonBlank = (urls: string[] | undefined) => (urls ?? []).filter((u) => u.trim() !== '');
const orBlank = (urls: string[] | undefined) => (urls && urls.length > 0 ? urls : ['']);
const sameList = (a: string[] | undefined, b: string[] | undefined) => nonBlank(a).join('\n') === nonBlank(b).join('\n');

function buildRequest(form: Omit<ModifyFormState, 'picks'>, picks: Picks): main.ModifyRequest {
  return main.ModifyRequest.createFrom({
    torrentPath: form.torrentPath,
    trackerUrls: picks.trackerMode === 'replace' ? nonBlank(picks.trackers) : [],
    removeTrackers: picks.trackerMode === 'remove' ? nonBlank(picks.removeTrackers) : [],
    noTracker: picks.trackerMode === 'clear',
    webSeeds: picks.webSeedMode === 'replace' ? nonBlank(picks.webSeeds) : [],
    noWebSeed: picks.webSeedMode === 'clear',
    name: form.name,
    isPrivate: picks.privateMode === 'private' ? true : picks.privateMode === 'public' ? false : undefined,
    noPrivate: picks.privateMode === 'remove',
    source: picks.source || undefined,
    comment: picks.comment || undefined,
    noDate: picks.noDate ?? false,
    noCreator: picks.noCreator ?? false,
    entropy: picks.entropyMode === 'add' ? true : picks.entropyMode === 'keep' ? false : undefined,
    noEntropy: picks.entropyMode === 'remove',
    skipPrefix: picks.skipPrefix ?? false,
    outputDir: picks.outputDir ?? '',
    outputPattern: form.outputPattern,
    presetName: form.presetName,
    presetFile: '',
    dryRun: false,
  });
}

function legacyPicks(saved: LegacyFormState): Picks {
  const trackerMode = saved.trackerMode ?? (nonBlank(saved.trackers).length > 0 ? 'replace' : 'keep');
  const webSeedMode = saved.webSeedMode ?? (nonBlank(saved.webSeeds).length > 0 ? 'replace' : 'keep');
  return {
    trackerMode: trackerMode === 'keep' ? undefined : trackerMode,
    trackers: saved.trackers,
    removeTrackers: saved.removeTrackers,
    webSeedMode: webSeedMode === 'keep' ? undefined : webSeedMode,
    webSeeds: saved.webSeeds,
    privateMode: saved.privateMode === 'unchanged' ? undefined : saved.privateMode,
    entropyMode: saved.entropyMode,
    source: saved.source || undefined,
    comment: saved.comment || undefined,
    outputDir: saved.outputDir || undefined,
    noDate: saved.noDate || undefined,
    noCreator: saved.noCreator || undefined,
    skipPrefix: saved.skipPrefix || undefined,
  };
}

async function migrateLegacy(form: Omit<ModifyFormState, 'picks'>, legacy: Picks): Promise<Picks> {
  const base: Picks = { outputDir: legacy.outputDir };
  const r = await ResolveModify(buildRequest(form, base));
  const picks: Picks = { ...base };
  if (legacy.trackerMode && !(legacy.trackerMode === 'replace' && r.trackers.action === 'set' && sameList(legacy.trackers, r.trackers.value))) {
    Object.assign(picks, { trackerMode: legacy.trackerMode, trackers: legacy.trackers, removeTrackers: legacy.removeTrackers });
  }
  if (legacy.webSeedMode && !(legacy.webSeedMode === 'replace' && r.webSeeds.action === 'set' && sameList(legacy.webSeeds, r.webSeeds.value))) {
    Object.assign(picks, { webSeedMode: legacy.webSeedMode, webSeeds: legacy.webSeeds });
  }
  if (legacy.privateMode && legacy.privateMode !== privateMode(r)) picks.privateMode = legacy.privateMode;
  if (legacy.entropyMode && legacy.entropyMode !== entropyMode(r)) picks.entropyMode = legacy.entropyMode;
  if (legacy.source && legacy.source !== r.source.value) picks.source = legacy.source;
  if (legacy.comment && legacy.comment !== r.comment.value) picks.comment = legacy.comment;
  if (legacy.noDate && r.creationDate.action !== 'clear') picks.noDate = true;
  if (legacy.noCreator && r.createdBy.action !== 'clear') picks.noCreator = true;
  if (legacy.skipPrefix && !r.skipPrefix.value) picks.skipPrefix = true;
  return picks;
}

function privateMode(r: ModifyResolution | undefined): PrivateMode {
  if (r?.private.action === 'clear') return 'remove';
  if (r?.private.action === 'set') return r.private.value ? 'private' : 'public';
  return 'unchanged';
}

function entropyMode(r: ModifyResolution | undefined): EntropyMode {
  if (r?.entropy.action === 'set') return 'add';
  if (r?.entropy.action === 'clear') return 'remove';
  return 'keep';
}

const STORAGE_KEY = 'mkbrr-modify-form';

function loadFormState(): ModifyFormState & { legacy?: Picks } {
  let saved: Partial<ModifyFormState> & LegacyFormState = {};
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    saved = raw ? JSON.parse(raw) : {};
  } catch (e) {
    console.error('Failed to load form state from localStorage:', e);
  }
  return {
    torrentPath: saved.torrentPath ?? '',
    presetName: saved.presetName ?? '',
    name: saved.name ?? '',
    outputPattern: saved.outputPattern ?? '',
    picks: saved.picks ?? {},
    legacy: saved.picks || Object.keys(saved).length === 0 ? undefined : legacyPicks(saved),
  };
}

function saveFormState(state: ModifyFormState) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch (e) {
    console.error('Failed to save form state to localStorage:', e);
  }
}

function clearFormState() {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch (e) {
    console.error('Failed to clear form state from localStorage:', e);
  }
}

export function ModifyPage() {
  const navigate = useNavigate();
  const [savedState] = useState(loadFormState);
  const [torrentPath, setTorrentPath] = useState(savedState.torrentPath);
  const [presetName, setPresetName] = useState(savedState.presetName);
  const [name, setName] = useState(savedState.name);
  const [outputPattern, setOutputPattern] = useState(savedState.outputPattern);
  const [picks, setPicks] = useState<Picks>(savedState.picks);
  const [legacy, setLegacy] = useState(savedState.legacy);
  const pick = (next: Picks) => setPicks((p) => ({ ...p, ...next }));
  const unpick = (...fields: (keyof Picks)[]) => setPicks((p) => {
    const next = { ...p };
    for (const field of fields) delete next[field];
    return next;
  });

  const [presets, setPresets] = useState<string[]>([]);
  const [isModifying, setIsModifying] = useState(false);
  const [result, setResult] = useState<ModifyResult | null>(null);
  const [error, setError] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [resolved, setResolved] = useState<{ presetName: string; resolution: ModifyResolution } | null>(null);
  const [resolveError, setResolveError] = useState('');
  const [presetFileRead, setPresetFileRead] = useState(0);

  // Drag-and-drop: accept .torrent files and populate the input field.
  const { isDragging } = useFileDrop((paths) => {
    const dropped = paths[0];
    if (!dropped.toLowerCase().endsWith('.torrent')) {
      toast.error('Please drop a .torrent file');
      return;
    }
    setTorrentPath(dropped);
    toast.success('Torrent file set');
  });

  const loadPresets = () => {
    ListPresets().then((names) => setPresets(names ?? [])).catch((e) => console.error('Failed to load presets:', e));
  };
  useEffect(loadPresets, []);

  useEffect(() => {
    const reread = () => {
      setPresetFileRead((n) => n + 1);
      loadPresets();
    };
    window.addEventListener('focus', reread);
    return () => window.removeEventListener('focus', reread);
  }, []);

  useEffect(() => {
    if (!legacy) return;
    migrateLegacy({ torrentPath, presetName, name, outputPattern }, legacy)
      .catch((e) => {
        console.error('Failed to migrate the saved form state:', e);
        return legacy;
      })
      .then((migrated) => {
        setPicks((current) => ({ ...migrated, ...current }));
        setLegacy(undefined);
      });
  }, []);

  useEffect(() => {
    if (legacy) return;
    saveFormState({ torrentPath, presetName, name, outputPattern, picks });
  }, [torrentPath, presetName, name, outputPattern, picks, legacy]);

  const request = buildRequest({ torrentPath, presetName, name, outputPattern }, picks);
  const resolveKey = JSON.stringify({ request, presetFileRead });
  useEffect(() => {
    if (legacy) return;
    let stale = false;
    ResolveModify(request)
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

  const resolution = !resolveError && resolved?.presetName === presetName ? resolved.resolution : undefined;
  const hint = (picked: unknown, change: { origin: string } | undefined) => (
    !picked && <OriginHint origin={change?.origin} presetName={presetName} />
  );

  const trackerMode: TrackerMode = picks.trackerMode ?? (resolution?.trackers.action === 'set' ? 'replace' : 'keep');
  const trackers = picks.trackerMode === 'replace' ? orBlank(picks.trackers) : orBlank(resolution?.trackers.value);
  const webSeedMode: WebSeedMode = picks.webSeedMode ?? (resolution?.webSeeds.action === 'set' ? 'replace' : 'keep');
  const webSeeds = picks.webSeedMode === 'replace' ? orBlank(picks.webSeeds) : orBlank(resolution?.webSeeds.value);
  const shownPrivateMode = picks.privateMode ?? privateMode(resolution);
  const shownEntropyMode = picks.entropyMode ?? entropyMode(resolution);
  const presetValue = (change: torrent.ModifyTextChange | undefined) => (change?.origin === 'preset' ? change.value : '');
  const presetOn = {
    noDate: resolution?.creationDate.action === 'clear',
    noCreator: resolution?.createdBy.action === 'clear',
    skipPrefix: resolution?.skipPrefix.value === true,
  };
  const toggleOrigin = {
    noDate: resolution?.creationDate,
    noCreator: resolution?.createdBy,
    skipPrefix: resolution?.skipPrefix,
  };
  const resolvedOutputDir = picks.outputDir ? '' : resolution?.outputDir.value ?? '';

  const handleTrackerMode = (mode: TrackerMode) => {
    if (mode === 'keep') unpick('trackerMode');
    else pick({ trackerMode: mode });
  };
  const handleWebSeedMode = (mode: WebSeedMode) => {
    if (mode === 'keep') unpick('webSeedMode');
    else pick({ webSeedMode: mode });
  };
  const handlePrivateMode = (mode: PrivateMode) => {
    if (mode === 'unchanged') unpick('privateMode');
    else pick({ privateMode: mode });
  };
  const handleToggle = (field: 'noDate' | 'noCreator' | 'skipPrefix', on: boolean) => {
    if (on) pick({ [field]: true });
    else unpick(field);
  };

  const handleReset = () => {
    setTorrentPath('');
    setPresetName('');
    setName('');
    setOutputPattern('');
    setPicks({});
    setResult(null);
    setError('');
    clearFormState();
  };

  const handlePresetChange = (value: string) => {
    setPresetName(value === 'none' ? '' : value);
  };

  const handleSelectInput = async () => {
    try {
      const path = await SelectTorrentFile();
      if (path) {
        setTorrentPath(path);
      }
    } catch (e) {
      setError(String(e));
    }
  };

  const handleSelectOutputDir = async () => {
    try {
      const path = await SelectPath();
      if (path) {
        pick({ outputDir: path });
      }
    } catch (e) {
      setError(String(e));
    }
  };

  const handleInspectResult = async () => {
    if (!result?.outputPath) return;
    try {
      const info = await InspectTorrent(result.outputPath);
      localStorage.setItem('mkbrr-inspect-state', JSON.stringify({ torrentInfo: info }));
      navigate('/inspect');
    } catch (e) {
      setError('Failed to inspect torrent: ' + String(e));
    }
  };

  const handleModify = async () => {
    if (!torrentPath) {
      setError('Please select a torrent file');
      return;
    }
    if (picks.trackerMode === 'replace' && nonBlank(picks.trackers).length === 0) {
      setError('Enter a tracker URL, or select Keep');
      return;
    }
    if (picks.trackerMode === 'remove' && nonBlank(picks.removeTrackers).length === 0) {
      setError('Enter a host or tracker URL to remove, or select Keep');
      return;
    }
    if (picks.webSeedMode === 'replace' && nonBlank(picks.webSeeds).length === 0) {
      setError('Enter a web seed URL, or select Keep');
      return;
    }

    setError('');
    setResult(null);
    setIsModifying(true);

    try {
      const res = await ModifyTorrent(request);
      setResult(res as ModifyResult);
    } catch (e) {
      setError(String(e));
    } finally {
      setIsModifying(false);
    }
  };

  return (
    <div className="flex flex-col h-full">
      <DropOverlay visible={isDragging} label="Drop a .torrent file to modify it" />
      <div className="flex-1 overflow-auto p-6 space-y-4">
        <div>
          <h1 className="text-2xl font-semibold">Modify Torrent</h1>
          <p className="text-sm text-muted-foreground">
            Modify torrent metadata without needing the original content
          </p>
        </div>

        {/* Main Form Card */}
        <Card>
          <CardContent className="pt-6 space-y-4">
            {/* Input Torrent */}
            <div className="space-y-1.5">
              <Label>Input Torrent</Label>
              <div className="flex gap-2">
                <div className="relative flex-1">
                  <Input
                    value={torrentPath}
                    onChange={(e) => setTorrentPath(e.target.value)}
                    placeholder="Select a .torrent file"
                    className={torrentPath ? 'pr-8' : ''}
                  />
                  {torrentPath && (
                    <button
                      type="button"
                      onClick={() => { setTorrentPath(''); setResult(null); setError(''); }}
                      className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                      aria-label="Clear"
                      title="Clear"
                    >
                      <X className="h-4 w-4" />
                    </button>
                  )}
                </div>
                <Button variant="outline" size="icon" onClick={handleSelectInput}>
                  <FolderOpen className="h-4 w-4" />
                </Button>
              </div>
            </div>

            {/* Preset Selector */}
            {presets.length > 0 && (
              <div className="space-y-1.5">
                <Label>Preset</Label>
                <Select value={presetName} onValueChange={handlePresetChange}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select a preset" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">None</SelectItem>
                    {presets.map((p) => (
                      <SelectItem key={p} value={p}>{p}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {/* Trackers */}
            <div className="space-y-1.5">
              <Label>Trackers</Label>
              <RadioRow
                name="trackers"
                value={trackerMode}
                onChange={handleTrackerMode}
                options={[['keep', 'Keep'], ['replace', 'Replace'], ['remove', 'Remove matching'], ['clear', 'Remove all']]}
              />
              {trackerMode === 'replace' && (
                <UrlListInput
                  urls={trackers}
                  onChange={(urls) => pick({ trackerMode: 'replace', trackers: urls })}
                  placeholder="https://tracker.example.com/announce"
                  addLabel="Add Tracker"
                />
              )}
              {trackerMode === 'remove' && (
                <UrlListInput
                  urls={orBlank(picks.removeTrackers)}
                  onChange={(urls) => pick({ removeTrackers: urls })}
                  placeholder="Host or tracker URL, for example old.example"
                  addLabel="Add Host"
                />
              )}
              {hint(picks.trackerMode, resolution?.trackers)}
            </div>

            {/* Private Flag */}
            <div className="space-y-1.5">
              <Label>Private Flag</Label>
              <RadioRow
                name="private"
                value={shownPrivateMode}
                onChange={handlePrivateMode}
                options={[['unchanged', 'Unchanged'], ['private', 'Private'], ['public', 'Public'], ['remove', 'Remove flag']]}
              />
              {hint(picks.privateMode, resolution?.private)}
            </div>

            {/* Output Directory */}
            <div className="space-y-1.5">
              <Label>Output Directory</Label>
              <div className="flex gap-2">
                <div className="relative flex-1">
                  <Input
                    value={picks.outputDir ?? ''}
                    onChange={(e) => (e.target.value ? pick({ outputDir: e.target.value }) : unpick('outputDir'))}
                    placeholder={resolvedOutputDir || 'Same as input file'}
                    className={picks.outputDir ? 'pr-8' : ''}
                  />
                  {picks.outputDir && (
                    <button
                      type="button"
                      onClick={() => unpick('outputDir')}
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
              {resolvedOutputDir && (
                <p className="text-xs text-muted-foreground">
                  Output: {resolvedOutputDir}
                </p>
              )}
              {hint(picks.outputDir, resolution?.outputDir)}
            </div>
            {resolveError && (
              <p className="text-xs text-destructive truncate" title={resolveError}>{resolveError}</p>
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
                {/* Source + Comment */}
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <Label>Source</Label>
                    <Input
                      value={picks.source ?? ''}
                      onChange={(e) => (e.target.value ? pick({ source: e.target.value }) : unpick('source'))}
                      placeholder={presetValue(resolution?.source) || 'Leave empty to keep unchanged'}
                    />
                    {hint(picks.source, resolution?.source)}
                  </div>
                  <div className="space-y-1.5">
                    <Label>Comment</Label>
                    <Input
                      value={picks.comment ?? ''}
                      onChange={(e) => (e.target.value ? pick({ comment: e.target.value }) : unpick('comment'))}
                      placeholder={presetValue(resolution?.comment) || 'Leave empty to keep unchanged'}
                    />
                    {hint(picks.comment, resolution?.comment)}
                  </div>
                </div>

                {/* Name + Output Filename */}
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <Label>Name</Label>
                    <Input
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="Leave empty to keep unchanged"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label>Output Filename</Label>
                    <Input
                      value={outputPattern}
                      onChange={(e) => setOutputPattern(e.target.value)}
                      placeholder="Without .torrent; empty uses the torrent name"
                    />
                  </div>
                </div>

                {/* Web Seeds */}
                <div className="space-y-1.5">
                  <Label>Web Seeds</Label>
                  <RadioRow
                    name="webSeeds"
                    value={webSeedMode}
                    onChange={handleWebSeedMode}
                    options={[['keep', 'Keep'], ['replace', 'Replace'], ['clear', 'Remove all']]}
                  />
                  {webSeedMode === 'replace' && (
                    <UrlListInput
                      urls={webSeeds}
                      onChange={(urls) => pick({ webSeedMode: 'replace', webSeeds: urls })}
                      placeholder="https://seed.example.com/files/"
                      addLabel="Add Web Seed"
                    />
                  )}
                  {hint(picks.webSeedMode, resolution?.webSeeds)}
                </div>

                {/* Entropy */}
                <div className="space-y-1.5">
                  <Label>Entropy</Label>
                  <RadioRow
                    name="entropy"
                    value={shownEntropyMode}
                    onChange={(mode) => pick({ entropyMode: mode })}
                    options={[['keep', 'Keep'], ['add', 'Add new'], ['remove', 'Remove']]}
                  />
                  {hint(picks.entropyMode, resolution?.entropy)}
                </div>

                {/* Toggles */}
                <div className="flex flex-wrap gap-6">
                  {([
                    ['noDate', 'Remove creation date'],
                    ['noCreator', 'Remove creator'],
                    ['skipPrefix', 'Skip tracker prefix in filename'],
                  ] as const).map(([field, label]) => (
                    <div key={field} className="space-y-1">
                      <div className="flex items-center gap-2">
                        <Switch
                          id={field}
                          checked={picks[field] ?? presetOn[field]}
                          disabled={!picks[field] && presetOn[field]}
                          onCheckedChange={(on) => handleToggle(field, on)}
                        />
                        <Label htmlFor={field} className="text-sm">{label}</Label>
                      </div>
                      {hint(picks[field], toggleOrigin[field])}
                    </div>
                  ))}
                </div>
              </div>
            </CollapsibleContent>
          </div>
        </Collapsible>

        {/* Error */}
        {error && (
          <Card className="border-destructive">
            <CardContent className="py-3">
              <p className="text-destructive text-sm">{error}</p>
            </CardContent>
          </Card>
        )}

        {/* Result */}
        {result && (
          <Card className="border-green-500">
            <CardContent className="py-4 space-y-2">
              <p className="font-medium text-green-600">Torrent Modified</p>
              <div className="grid grid-cols-[80px_1fr] gap-x-2 gap-y-1 text-sm">
                <span className="text-muted-foreground">Output</span>
                <span className="font-mono text-xs break-all">{result.outputPath}</span>
                <span className="text-muted-foreground">Status</span>
                <span>{result.wasModified ? 'Modified successfully' : 'No changes made'}</span>
              </div>
              {result.notices?.map((notice, i) => (
                <p key={i} className={`text-sm ${notice.warn ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground'}`}>{notice.text}</p>
              ))}
              <div className="pt-2">
                <Button variant="outline" size="sm" onClick={handleInspectResult}>
                  <FileSearch className="mr-2 h-4 w-4" />
                  Inspect
                </Button>
              </div>
            </CardContent>
          </Card>
        )}
      </div>

      <div className="bg-background p-4 flex justify-end gap-2">
        <Button variant="outline" onClick={handleReset} disabled={isModifying || Boolean(legacy)}>
          Reset
        </Button>
        <Button onClick={handleModify} disabled={isModifying || !torrentPath || Boolean(legacy)}>
          {isModifying && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {isModifying ? 'Modifying...' : 'Modify Torrent'}
        </Button>
      </div>
    </div>
  );
}
