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
import { SelectTorrentFile, SelectPath, ModifyTorrent, ListPresets, GetPreset, InspectTorrent } from '../../wailsjs/go/main/App';
import { useFileDrop } from '@/hooks/useFileDrop';
import { DropOverlay } from '@/components/ui/drop-overlay';

import { main, preset as presetTypes } from '../../wailsjs/go/models';

type ModifyRequest = main.ModifyRequest;
type ModifyResult = main.ModifyResult;
type PresetOptions = presetTypes.Options;

// Get directory from path (works for both Unix and Windows paths)
function getDirectory(path: string): string {
  if (!path || typeof path !== 'string') return '';
  // Handle both forward and backslashes
  const lastSlash = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
  return lastSlash > 0 ? path.substring(0, lastSlash) : path;
}

type PrivateMode = 'unchanged' | 'private' | 'public' | 'remove';
type EntropyMode = 'keep' | 'add' | 'remove';

// Form state persistence
interface ModifyFormState {
  torrentPath: string;
  outputDir: string;
  outputPattern: string;
  trackers: string[];
  webSeeds: string[];
  name: string;
  privateMode: PrivateMode;
  entropyMode: EntropyMode;
  source: string;
  comment: string;
  noDate: boolean;
  noCreator: boolean;
  skipPrefix: boolean;
  presetName: string;
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

function nonEmptyList(list: unknown): string[] {
  return Array.isArray(list) && list.length > 0 ? list : [''];
}

const STORAGE_KEY = 'mkbrr-modify-form';

function loadFormState(): Partial<ModifyFormState> {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    return saved ? JSON.parse(saved) : {};
  } catch (e) {
    console.error('Failed to load form state from localStorage:', e);
    return {};
  }
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
  const savedState = loadFormState();
  const [torrentPath, setTorrentPath] = useState(savedState.torrentPath ?? '');
  const [outputDir, setOutputDir] = useState(savedState.outputDir ?? '');
  const [outputPattern, setOutputPattern] = useState(savedState.outputPattern ?? '');
  const [trackers, setTrackers] = useState(nonEmptyList(savedState.trackers));
  const [webSeeds, setWebSeeds] = useState(nonEmptyList(savedState.webSeeds));
  const [name, setName] = useState(savedState.name ?? '');
  const [privateMode, setPrivateMode] = useState<PrivateMode>(savedState.privateMode ?? 'unchanged');
  const [entropyMode, setEntropyMode] = useState<EntropyMode>(savedState.entropyMode ?? 'keep');
  const [source, setSource] = useState(savedState.source ?? '');
  const [comment, setComment] = useState(savedState.comment ?? '');
  const [noDate, setNoDate] = useState(savedState.noDate ?? false);
  const [noCreator, setNoCreator] = useState(savedState.noCreator ?? false);
  const [skipPrefix, setSkipPrefix] = useState(savedState.skipPrefix ?? false);
  const [presetName, setPresetName] = useState(savedState.presetName ?? '');

  const [presets, setPresets] = useState<string[]>([]);
  const [isModifying, setIsModifying] = useState(false);
  const [result, setResult] = useState<ModifyResult | null>(null);
  const [error, setError] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);

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

  // Load presets on mount
  useEffect(() => {
    ListPresets().then(setPresets).catch((e) => console.error('Failed to load presets:', e));
  }, []);

  // Save form state to localStorage whenever values change
  useEffect(() => {
    saveFormState({
      torrentPath,
      outputDir,
      outputPattern,
      trackers,
      webSeeds,
      name,
      privateMode,
      entropyMode,
      source,
      comment,
      noDate,
      noCreator,
      skipPrefix,
      presetName,
    });
  }, [torrentPath, outputDir, outputPattern, trackers, webSeeds, name, privateMode, entropyMode, source, comment, noDate, noCreator, skipPrefix, presetName]);

  const handleReset = () => {
    setTorrentPath('');
    setOutputDir('');
    setOutputPattern('');
    setTrackers(['']);
    setWebSeeds(['']);
    setName('');
    setPrivateMode('unchanged');
    setEntropyMode('keep');
    setSource('');
    setComment('');
    setNoDate(false);
    setNoCreator(false);
    setSkipPrefix(false);
    setPresetName('');
    setResult(null);
    setError('');
    clearFormState();
  };

  const handlePresetChange = async (value: string) => {
    if (value === 'none') {
      setPresetName('');
      return;
    }

    setPresetName(value);
    try {
      const preset = await GetPreset(value) as PresetOptions;
      if (preset) {
        // Support both camelCase (new) and PascalCase (legacy) field names
        const trackerList = preset.trackers || (preset as any).Trackers;
        const sourceVal = preset.source || (preset as any).Source;
        const privateVal = preset.private ?? (preset as any).Private;
        const commentVal = preset.comment || (preset as any).Comment;

        if (trackerList && trackerList.length > 0) setTrackers(trackerList);
        if (sourceVal) setSource(sourceVal);
        if (privateVal !== undefined) setPrivateMode(privateVal ? 'private' : 'public');
        if (commentVal) setComment(commentVal);
        // "Keep" sends entropy false, which wins over the preset, so show the preset value
        if (preset.entropy) setEntropyMode('add');
      }
    } catch (e) {
      setError('Failed to load preset: ' + String(e));
    }
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
        setOutputDir(path);
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

    setError('');
    setResult(null);
    setIsModifying(true);

    try {
      const req: ModifyRequest = {
        torrentPath,
        trackerUrls: trackers.filter(t => t.trim() !== ''),
        webSeeds: webSeeds.filter(w => w.trim() !== ''),
        name,
        isPrivate: privateMode === 'private' ? true : privateMode === 'public' ? false : undefined,
        noPrivate: privateMode === 'remove',
        // an empty string clears, so send nothing to keep
        source: source || undefined,
        comment: comment || undefined,
        noDate,
        noCreator,
        // false keeps the entropy, also over a preset
        entropy: entropyMode === 'add',
        noEntropy: entropyMode === 'remove',
        skipPrefix,
        outputDir,
        outputPattern,
        presetName,
        presetFile: '',
        dryRun: false,
      };

      const res = await ModifyTorrent(req);
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
              <Label>Add Trackers</Label>
              <UrlListInput
                urls={trackers}
                onChange={setTrackers}
                placeholder="https://tracker.example.com/announce"
                addLabel="Add Tracker"
              />
            </div>

            {/* Private Flag */}
            <div className="space-y-1.5">
              <Label>Private Flag</Label>
              <RadioRow
                name="private"
                value={privateMode}
                onChange={setPrivateMode}
                options={[['unchanged', 'Unchanged'], ['private', 'Private'], ['public', 'Public'], ['remove', 'Remove flag']]}
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
                    placeholder={getDirectory(torrentPath) || 'Same as input file'}
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
              {torrentPath && !outputDir && (
                <p className="text-xs text-muted-foreground">
                  Output: {getDirectory(torrentPath)}
                </p>
              )}
            </div>
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
                      value={source}
                      onChange={(e) => setSource(e.target.value)}
                      placeholder="Leave empty to keep unchanged"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label>Comment</Label>
                    <Input
                      value={comment}
                      onChange={(e) => setComment(e.target.value)}
                      placeholder="Leave empty to keep unchanged"
                    />
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
                  <UrlListInput
                    urls={webSeeds}
                    onChange={setWebSeeds}
                    placeholder="Leave empty to keep unchanged"
                    addLabel="Add Web Seed"
                  />
                </div>

                {/* Entropy */}
                <div className="space-y-1.5">
                  <Label>Entropy</Label>
                  <RadioRow
                    name="entropy"
                    value={entropyMode}
                    onChange={setEntropyMode}
                    options={[['keep', 'Keep'], ['add', 'Add new'], ['remove', 'Remove']]}
                  />
                </div>

                {/* Toggles */}
                <div className="flex flex-wrap gap-6">
                  <div className="flex items-center gap-2">
                    <Switch
                      id="noDate"
                      checked={noDate}
                      onCheckedChange={setNoDate}
                    />
                    <Label htmlFor="noDate" className="text-sm">Remove creation date</Label>
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="noCreator"
                      checked={noCreator}
                      onCheckedChange={setNoCreator}
                    />
                    <Label htmlFor="noCreator" className="text-sm">Remove creator</Label>
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="skipPrefix"
                      checked={skipPrefix}
                      onCheckedChange={setSkipPrefix}
                    />
                    <Label htmlFor="skipPrefix" className="text-sm">Skip tracker prefix in filename</Label>
                  </div>
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
        <Button variant="outline" onClick={handleReset} disabled={isModifying}>
          Reset
        </Button>
        <Button onClick={handleModify} disabled={isModifying || !torrentPath}>
          {isModifying && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {isModifying ? 'Modifying...' : 'Modify Torrent'}
        </Button>
      </div>
    </div>
  );
}
