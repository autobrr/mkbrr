import { useState, useEffect } from 'react';
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
import { FolderOpen, File, Plus, X, Loader2, ChevronDown, Sparkles } from 'lucide-react';
import { SelectPath, SelectFile, CreateTorrent, ListPresets, GetPreset, GetTrackerInfo, GetContentSize, GetRecommendedPieceSize } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

import { main, preset as presetTypes } from '../../wailsjs/go/models';

// Re-export types from generated models
type CreateRequest = main.CreateRequest;
type TorrentResultType = main.TorrentResult;
type PresetOptions = presetTypes.Options;
type TrackerInfoType = main.TrackerInfo;

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
  isPrivate: boolean;
  comment: string;
  source: string;
  pieceLengthExp: number;
  outputDir: string;
  noDate: boolean;
  noCreator: boolean;
  entropy: boolean;
  presetName: string;
}

const STORAGE_KEY = 'mkbrr-create-form';
const RESULT_STORAGE_KEY = 'mkbrr-create-result';

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

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export function CreatePage() {
  // Load saved form state from localStorage
  const savedState = loadFormState();

  const [path, setPath] = useState(savedState.path ?? '');
  const [name, setName] = useState(savedState.name ?? '');
  const [trackers, setTrackers] = useState<string[]>(savedState.trackers ?? ['']);
  const [isPrivate, setIsPrivate] = useState(savedState.isPrivate ?? true);
  const [comment, setComment] = useState(savedState.comment ?? '');
  const [source, setSource] = useState(savedState.source ?? '');
  const [pieceLengthExp, setPieceLengthExp] = useState(savedState.pieceLengthExp ?? 0);
  const [outputDir, setOutputDir] = useState(savedState.outputDir ?? '');
  const [noDate, setNoDate] = useState(savedState.noDate ?? false);
  const [noCreator, setNoCreator] = useState(savedState.noCreator ?? false);
  const [entropy, setEntropy] = useState(savedState.entropy ?? false);
  const [presetName, setPresetName] = useState(savedState.presetName ?? '');

  const [presets, setPresets] = useState<string[]>([]);
  const [isCreating, setIsCreating] = useState(false);
  const [progress, setProgress] = useState<ProgressEvent | null>(null);
  const [result, setResult] = useState<TorrentResultType | null>(null);
  const [error, setError] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [trackerInfo, setTrackerInfo] = useState<TrackerInfoType | null>(null);
  const [contentSize, setContentSize] = useState<number>(0);
  const [recommendedPieceSize, setRecommendedPieceSize] = useState<number>(0);
  const [dialogOpen, setDialogOpen] = useState(false);

  useEffect(() => {
    ListPresets().then(setPresets).catch((e) => toast.error('Failed to load presets: ' + String(e)));
  }, []);

  // Save form state to localStorage whenever values change
  useEffect(() => {
    saveFormState({
      path,
      name,
      trackers,
      isPrivate,
      comment,
      source,
      pieceLengthExp,
      outputDir,
      noDate,
      noCreator,
      entropy,
      presetName,
    });
  }, [path, name, trackers, isPrivate, comment, source, pieceLengthExp, outputDir, noDate, noCreator, entropy, presetName]);

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

  // Check trackers for known tracker configurations
  useEffect(() => {
    const checkTrackers = async () => {
      const validTrackers = trackers.filter(t => t.trim() !== '');
      if (validTrackers.length === 0) {
        setTrackerInfo(null);
        return;
      }

      // Check each tracker URL
      for (const tracker of validTrackers) {
        try {
          const info = await GetTrackerInfo(tracker);
          if (info && info.hasCustomRules) {
            setTrackerInfo(info);
            return;
          }
        } catch (e) {
          toast.error('Failed to get tracker info: ' + String(e));
        }
      }
      setTrackerInfo(null);
    };

    const debounce = setTimeout(checkTrackers, 300);
    return () => clearTimeout(debounce);
  }, [trackers]);

  // Get content size when path changes
  useEffect(() => {
    if (!path) {
      setContentSize(0);
      return;
    }

    const fetchSize = async () => {
      try {
        const size = await GetContentSize(path);
        setContentSize(size);
      } catch (e) {
        toast.error('Failed to get content size: ' + String(e));
        setContentSize(0);
      }
    };

    const debounce = setTimeout(fetchSize, 300);
    return () => clearTimeout(debounce);
  }, [path]);

  // Calculate recommended piece size when tracker info and content size are available
  useEffect(() => {
    const calculatePieceSize = async () => {
      if (!trackerInfo?.hasCustomRules || contentSize === 0) {
        setRecommendedPieceSize(0);
        return;
      }

      const validTrackers = trackers.filter(t => t.trim() !== '');
      for (const tracker of validTrackers) {
        try {
          const exp = await GetRecommendedPieceSize(tracker, contentSize);
          if (exp > 0) {
            setRecommendedPieceSize(exp);
            return;
          }
        } catch (e) {
          toast.error('Failed to get recommended piece size: ' + String(e));
        }
      }
      setRecommendedPieceSize(0);
    };

    calculatePieceSize();
  }, [trackerInfo, contentSize, trackers]);

  // Reset piece length if it exceeds tracker's max
  useEffect(() => {
    if (trackerInfo?.maxPieceLength && pieceLengthExp > trackerInfo.maxPieceLength) {
      setPieceLengthExp(0); // Reset to Auto
    }
  }, [trackerInfo?.maxPieceLength, pieceLengthExp]);

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
    setIsPrivate(true);
    setComment('');
    setSource('');
    setPieceLengthExp(0);
    setOutputDir('');
    setNoDate(false);
    setNoCreator(false);
    setEntropy(false);
    setPresetName('');
    setResult(null);
    setError('');
    setTrackerInfo(null);
    setContentSize(0);
    setRecommendedPieceSize(0);
    clearFormState(); // Also clear localStorage
  };

  const handlePresetChange = async (value: string) => {
    if (value === 'none') {
      // Clear preset-related fields when "None" is selected
      setPresetName('');
      setTrackers(['']);
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
          if (preset.Trackers && preset.Trackers.length > 0) setTrackers(preset.Trackers);
          if (preset.Source) setSource(preset.Source);
          if (preset.Private !== undefined) setIsPrivate(preset.Private);
          if (preset.Comment) setComment(preset.Comment);
        }
      } catch (e) {
        toast.error('Failed to load preset: ' + String(e));
      }
    }
  };

  const addTracker = () => {
    setTrackers([...trackers, '']);
  };

  const removeTracker = (index: number) => {
    setTrackers(trackers.filter((_, i) => i !== index));
  };

  const updateTracker = (index: number, value: string) => {
    const newTrackers = [...trackers];
    newTrackers[index] = value;
    setTrackers(newTrackers);
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
      const req: CreateRequest = {
        path,
        name,
        trackerUrls: trackers.filter(t => t.trim() !== ''),
        webSeeds: [],
        isPrivate,
        comment,
        source,
        pieceLengthExp,
        maxPieceLength: 0,
        outputPath: '',
        outputDir,
        noDate,
        noCreator,
        entropy,
        skipPrefix: false,
        excludePatterns: [],
        includePatterns: [],
        presetName,
        presetFile: '',
      };

      const res = await CreateTorrent(req);
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
                <Input
                  value={path}
                  onChange={(e) => setPath(e.target.value)}
                  placeholder="/path/to/file/or/folder"
                  className="flex-1"
                />
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
              <div className="space-y-2">
                {trackers.map((tracker, index) => (
                  <div key={index} className="flex gap-2">
                    <Input
                      value={tracker}
                      onChange={(e) => updateTracker(index, e.target.value)}
                      placeholder="https://tracker.example.com/announce"
                      className="flex-1"
                    />
                    {trackers.length > 1 && (
                      <Button
                        variant="outline"
                        size="icon"
                        onClick={() => removeTracker(index)}
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    )}
                  </div>
                ))}
                <Button variant="outline" size="sm" onClick={addTracker}>
                  <Plus className="mr-2 h-4 w-4" />
                  Add Tracker
                </Button>
              </div>
            </div>

            {/* Output Directory */}
            <div className="space-y-1.5">
              <Label>Output Directory</Label>
              <div className="flex gap-2">
                <Input
                  value={outputDir}
                  onChange={(e) => setOutputDir(e.target.value)}
                  placeholder="Same as source"
                  className="flex-1"
                />
                <Button variant="outline" size="icon" onClick={handleSelectOutputDir}>
                  <FolderOpen className="h-4 w-4" />
                </Button>
              </div>
            </div>

            {/* Tracker Optimization Indicator */}
            {trackerInfo && trackerInfo.hasCustomRules && (
              <div className="flex items-start gap-3 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20">
                <Sparkles className="h-4 w-4 text-emerald-500 mt-0.5 flex-shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-4">
                    <p className="text-sm font-medium text-emerald-600 dark:text-emerald-400">
                      Tracker optimizations detected
                    </p>
                    {recommendedPieceSize > 0 && (
                      <div className="flex items-center gap-2 px-2.5 py-1 rounded-md bg-emerald-500/20">
                        <span className="text-xs text-emerald-600 dark:text-emerald-400">Piece size:</span>
                        {pieceLengthExp > 0 ? (
                          <span className="text-sm font-bold text-amber-600 dark:text-amber-400 line-through">{formatPieceSize(recommendedPieceSize)}</span>
                        ) : (
                          <span className="text-sm font-bold text-emerald-700 dark:text-emerald-300">{formatPieceSize(recommendedPieceSize)}</span>
                        )}
                      </div>
                    )}
                  </div>
                  {pieceLengthExp > 0 && (
                    <p className="text-xs text-amber-600 dark:text-amber-400 mt-1">
                      Manual override: using {formatPieceSize(pieceLengthExp)} instead
                    </p>
                  )}
                  <div className="flex flex-wrap gap-x-4 gap-y-1 mt-1.5 text-xs text-muted-foreground">
                    {trackerInfo.defaultSource && (
                      <span>Source: <span className="font-medium text-foreground">{trackerInfo.defaultSource}</span></span>
                    )}
                    {trackerInfo.maxPieceLength > 0 && (
                      <span>Max piece: <span className="font-medium text-foreground">{formatPieceSize(trackerInfo.maxPieceLength)}</span></span>
                    )}
                    {trackerInfo.maxTorrentSize > 0 && (
                      <span>Max .torrent: <span className="font-medium text-foreground">{formatBytes(trackerInfo.maxTorrentSize)}</span></span>
                    )}
                    {contentSize > 0 && (
                      <span>Content: <span className="font-medium text-foreground">{formatBytes(contentSize)}</span></span>
                    )}
                  </div>
                </div>
              </div>
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

                {/* Piece Length */}
                <div className="space-y-1.5">
                  <Label>Piece Length</Label>
                  <Select
                    value={pieceLengthExp.toString()}
                    onValueChange={(v) => setPieceLengthExp(parseInt(v))}
                  >
                    <SelectTrigger className="w-48">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {PIECE_LENGTHS
                        .filter((pl) => {
                          // If tracker has max piece length limit, filter out larger sizes
                          if (trackerInfo?.maxPieceLength && pl.value > 0) {
                            return pl.value <= trackerInfo.maxPieceLength;
                          }
                          return true;
                        })
                        .map((pl) => (
                          <SelectItem key={pl.value} value={pl.value.toString()}>
                            {pl.label}
                          </SelectItem>
                        ))}
                    </SelectContent>
                  </Select>
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
