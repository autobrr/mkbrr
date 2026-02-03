import { useState, useEffect } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Separator } from '@/components/ui/separator';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { RefreshCw, Plus, Pencil, Trash2, X, ChevronDown } from 'lucide-react';
import {
  GetPresetFilePath,
  GetAllPresets,
  SavePreset,
  DeletePreset,
  CreatePresetFile,
} from '../../wailsjs/go/main/App';
import { preset } from '../../wailsjs/go/models';

type PresetOptions = preset.Options;

// === Default Settings Storage ===
const DEFAULT_SETTINGS_KEY = 'mkbrr-default-settings';

export interface DefaultSettings {
  workers: number;
}

const DEFAULT_SETTINGS: DefaultSettings = {
  workers: 1,
};

export function loadDefaultSettings(): DefaultSettings {
  try {
    const saved = localStorage.getItem(DEFAULT_SETTINGS_KEY);
    if (saved) {
      const parsed = JSON.parse(saved);
      return { ...DEFAULT_SETTINGS, ...parsed };
    }
  } catch (e) {
    console.error('Failed to load default settings:', e);
  }
  return DEFAULT_SETTINGS;
}

export function saveDefaultSettings(settings: DefaultSettings): void {
  try {
    localStorage.setItem(DEFAULT_SETTINGS_KEY, JSON.stringify(settings));
  } catch (e) {
    console.error('Failed to save default settings:', e);
  }
}

/**
 * Get the effective workers count, considering preset override and default settings.
 * @param presetWorkers - Workers value from a preset (0 means "use default")
 * @returns The effective number of workers to use
 */
export function getEffectiveWorkers(presetWorkers?: number): number {
  const defaults = loadDefaultSettings();
  // If preset specifies workers > 0, use that; otherwise use default
  if (presetWorkers && presetWorkers > 0) {
    return presetWorkers;
  }
  return defaults.workers;
}

interface PresetFormData {
  name: string;
  source: string;
  comment: string;
  isPrivate: boolean;
  noDate: boolean;
  noCreator: boolean;
  entropy: boolean;
  skipPrefix: boolean;
  trackers: string[];
  pieceLength: number;
  maxPieceLength: number;
  workers: number;
}

const emptyFormData: PresetFormData = {
  name: '',
  source: '',
  comment: '',
  isPrivate: true,
  noDate: false,
  noCreator: false,
  entropy: false,
  skipPrefix: false,
  trackers: [''],
  pieceLength: 0,
  maxPieceLength: 0,
  workers: 0, // 0 = use default from settings
};

// Preset name validation constants (must match backend)
const MAX_PRESET_NAME_LENGTH = 64;
const INVALID_PRESET_NAME_CHARS = /[/\\:*?"<>|]/;

function validatePresetName(name: string): string | null {
  const trimmed = name.trim();
  if (!trimmed) {
    return 'Preset name cannot be empty';
  }
  if (trimmed.length > MAX_PRESET_NAME_LENGTH) {
    return `Preset name too long (max ${MAX_PRESET_NAME_LENGTH} characters)`;
  }
  if (INVALID_PRESET_NAME_CHARS.test(trimmed)) {
    return 'Preset name contains invalid characters';
  }
  return null;
}

function optionsToFormData(name: string, options: PresetOptions): PresetFormData {
  // Support both camelCase (new) and PascalCase (legacy) field names
  return {
    name,
    source: options.source || (options as any).Source || '',
    comment: options.comment || (options as any).Comment || '',
    isPrivate: options.private ?? (options as any).Private ?? true,
    noDate: options.noDate ?? (options as any).NoDate ?? false,
    noCreator: options.noCreator ?? (options as any).NoCreator ?? false,
    entropy: options.entropy ?? (options as any).Entropy ?? false,
    skipPrefix: options.skipPrefix ?? (options as any).SkipPrefix ?? false,
    trackers: (options.trackers && options.trackers.length > 0)
      ? options.trackers
      : ((options as any).Trackers && (options as any).Trackers.length > 0)
        ? (options as any).Trackers
        : [''],
    pieceLength: options.pieceLength || (options as any).PieceLength || 0,
    maxPieceLength: options.maxPieceLength || (options as any).MaxPieceLength || 0,
    workers: options.workers ?? (options as any).Workers ?? 0, // 0 = use default
  };
}

function formDataToOptions(data: PresetFormData): PresetOptions {
  const options = new preset.Options();
  // Use camelCase field names (matching new JSON tags)
  options.private = data.isPrivate;
  options.noDate = data.noDate;
  options.noCreator = data.noCreator;
  options.entropy = data.entropy;
  options.skipPrefix = data.skipPrefix;
  options.source = data.source;
  options.comment = data.comment;
  options.trackers = data.trackers.filter(t => t.trim() !== '');
  options.pieceLength = data.pieceLength;
  options.maxPieceLength = data.maxPieceLength;
  options.webSeeds = [];
  options.excludePatterns = [];
  options.includePatterns = [];
  options.outputDir = '';
  options.workers = data.workers;
  return options;
}

export function SettingsPage() {
  const [presetPath, setPresetPath] = useState('');
  const [presets, setPresets] = useState<Record<string, PresetOptions>>({});
  const [presetErrors, setPresetErrors] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  // Default settings state
  const [defaultWorkers, setDefaultWorkers] = useState(() => loadDefaultSettings().workers);

  // Dialog states
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [presetToDelete, setPresetToDelete] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [originalName, setOriginalName] = useState<string | null>(null);
  const [formData, setFormData] = useState<PresetFormData>(emptyFormData);
  const [nameError, setNameError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);

  useEffect(() => {
    loadSettings();
  }, []);

  // Save default settings when they change
  const handleDefaultWorkersChange = (value: number) => {
    const newValue = Math.max(1, Math.min(32, value || 1)); // Clamp between 1-32
    setDefaultWorkers(newValue);
    saveDefaultSettings({ workers: newValue });
  };

  const loadSettings = async () => {
    setIsLoading(true);
    try {
      const [pathResult, presetMapResult] = await Promise.allSettled([
        GetPresetFilePath(),
        GetAllPresets(),
      ]);

      if (pathResult.status === 'fulfilled') {
        setPresetPath(pathResult.value);
      } else {
        console.error('Failed to get preset file path:', pathResult.reason);
        setPresetPath('');
      }

      if (presetMapResult.status === 'fulfilled') {
        const result = presetMapResult.value as { presets: Record<string, PresetOptions>; errors?: string[] };
        setPresets(result.presets || {});
        setPresetErrors(result.errors || []);
        // Show toast for any preset loading errors
        if (result.errors && result.errors.length > 0) {
          toast.warning(`${result.errors.length} preset(s) failed to load. Check settings for details.`);
        }
      } else {
        console.error('Failed to load presets:', presetMapResult.reason);
        toast.error('Failed to load presets: ' + String(presetMapResult.reason));
        setPresets({});
        setPresetErrors([]);
      }
    } catch (e) {
      console.error('Failed to load settings:', e);
      toast.error('Failed to load settings: ' + String(e));
    } finally {
      setIsLoading(false);
    }
  };

  const handleNewPreset = async () => {
    // Ensure preset file exists
    if (!presetPath) {
      try {
        const path = await CreatePresetFile();
        setPresetPath(path);
      } catch (e) {
        toast.error('Failed to create preset file: ' + String(e));
        // Still allow opening the dialog - save will create the file
      }
    }
    setFormData(emptyFormData);
    setIsEditing(false);
    setOriginalName(null);
    setAdvancedOpen(false);
    setNameError(null);
    setEditDialogOpen(true);
  };

  const handleEditPreset = (name: string) => {
    const options = presets[name];
    if (options) {
      setFormData(optionsToFormData(name, options));
      setIsEditing(true);
      setOriginalName(name);
      setAdvancedOpen(false);
      setNameError(null);
      setEditDialogOpen(true);
    }
  };

  const handleDeleteClick = (name: string) => {
    setPresetToDelete(name);
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (!presetToDelete) return;
    try {
      await DeletePreset(presetToDelete);
      await loadSettings();
      // Only close dialog and reset state on success
      setDeleteDialogOpen(false);
      setPresetToDelete(null);
    } catch (e) {
      toast.error('Failed to delete preset: ' + String(e));
      // Keep dialog open so user can try again or cancel
    }
  };

  const handleSavePreset = async () => {
    // Validate preset name
    const validationError = validatePresetName(formData.name);
    if (validationError) {
      setNameError(validationError);
      return;
    }
    setNameError(null);

    setIsSaving(true);
    try {
      // If renaming, we need to handle this atomically
      // First save the new preset, then delete the old one
      const options = formDataToOptions(formData);
      await SavePreset(formData.name.trim(), options);

      // If renaming, delete old preset after new one is saved
      if (isEditing && originalName && originalName !== formData.name.trim()) {
        try {
          await DeletePreset(originalName);
        } catch (e) {
          // New preset was saved, but old one couldn't be deleted
          // This is not critical - warn user but don't fail
          toast.warning(`Preset saved, but failed to remove old preset "${originalName}": ${String(e)}`);
        }
      }

      await loadSettings();
      setEditDialogOpen(false);
    } catch (e) {
      toast.error('Failed to save preset: ' + String(e));
    } finally {
      setIsSaving(false);
    }
  };

  const addTracker = () => {
    setFormData({ ...formData, trackers: [...formData.trackers, ''] });
  };

  const removeTracker = (index: number) => {
    setFormData({
      ...formData,
      trackers: formData.trackers.filter((_, i) => i !== index),
    });
  };

  const updateTracker = (index: number, value: string) => {
    const newTrackers = [...formData.trackers];
    newTrackers[index] = value;
    setFormData({ ...formData, trackers: newTrackers });
  };

  const presetNames = Object.keys(presets).sort();

  return (
    <div className="flex flex-col h-full overflow-auto">
      <div className="flex-1 p-6 space-y-4">
        <div>
          <h1 className="text-2xl font-semibold">Settings</h1>
          <p className="text-sm text-muted-foreground">Manage default settings and presets</p>
        </div>

        {/* Default Settings Card */}
        <Card>
          <CardHeader className="py-3">
            <CardTitle className="text-base">Default Settings</CardTitle>
            <CardDescription className="text-xs">
              These settings apply when no preset is selected, or when a preset doesn't specify a value.
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-0 space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="default-workers">Default Workers</Label>
              <Input
                id="default-workers"
                type="number"
                min={1}
                max={32}
                value={defaultWorkers}
                onChange={(e) => handleDefaultWorkersChange(parseInt(e.target.value) || 1)}
                className="w-32"
              />
              <p className="text-xs text-muted-foreground">
                Number of parallel workers for hashing. Used when creating torrents without a preset,
                or when the preset doesn't override this value.
              </p>
            </div>
          </CardContent>
        </Card>

        <Card className="flex-1">
            <CardHeader className="py-3 flex flex-row items-center justify-between">
              <CardTitle className="text-base">Presets</CardTitle>
              <Button variant="outline" size="sm" onClick={handleNewPreset}>
                <Plus className="mr-2 h-4 w-4" />
                New Preset
              </Button>
            </CardHeader>
            <CardContent className="pt-0 space-y-3">
              <div className="space-y-1.5">
                <Label className="text-xs">Preset File Location</Label>
                <div className="flex gap-2">
                  <Input
                    value={presetPath || 'No preset file found'}
                    readOnly
                    className="flex-1 font-mono text-xs h-8"
                  />
                  <Button variant="outline" size="sm" onClick={loadSettings} disabled={isLoading}>
                    <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
                  </Button>
                </div>
              </div>
              <Separator />
              {presetErrors.length > 0 && (
                <div className="rounded border border-amber-500 bg-amber-500/10 p-3 space-y-1">
                  <p className="text-xs font-medium text-amber-600 dark:text-amber-400">
                    Failed to load {presetErrors.length} preset(s):
                  </p>
                  <ul className="text-xs text-amber-600 dark:text-amber-400 list-disc list-inside">
                    {presetErrors.map((err, i) => (
                      <li key={i}>{err}</li>
                    ))}
                  </ul>
                </div>
              )}
              <div className="space-y-1.5">
                <Label className="text-xs">Available Presets ({presetNames.length})</Label>
                {presetNames.length === 0 && presetErrors.length === 0 ? (
                  <p className="text-xs text-muted-foreground py-4 text-center">
                    No presets configured. Click "New Preset" to create one.
                  </p>
                ) : (
                  <div className="space-y-2">
                      {presetNames.map((name) => {
                        const opts = presets[name];
                        return (
                          <div
                            key={name}
                            className="rounded border bg-muted/30 p-3 space-y-2"
                          >
                            <div className="flex items-center justify-between">
                              <span className="font-medium text-sm">{name}</span>
                              <div className="flex gap-1">
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-7 w-7"
                                  onClick={() => handleEditPreset(name)}
                                >
                                  <Pencil className="h-3.5 w-3.5" />
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-7 w-7 text-destructive hover:text-destructive"
                                  onClick={() => handleDeleteClick(name)}
                                >
                                  <Trash2 className="h-3.5 w-3.5" />
                                </Button>
                              </div>
                            </div>
                            <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                              {(opts.source || (opts as any).Source) && <span>Source: {opts.source || (opts as any).Source}</span>}
                              <span>Private: {(opts.private ?? (opts as any).Private) ? 'Yes' : 'No'}</span>
                              {((opts.trackers && opts.trackers.length > 0) || ((opts as any).Trackers && (opts as any).Trackers.length > 0)) && (
                                <span>Trackers: {opts.trackers?.length || (opts as any).Trackers?.length}</span>
                              )}
                              <span>Workers: {(opts.workers ?? (opts as any).Workers) || `default (${defaultWorkers})`}</span>
                            </div>
                          </div>
                        );
                      })}
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
      </div>

      {/* Edit/Create Preset Dialog */}
      <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
        <DialogContent className="max-w-md max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{isEditing ? 'Edit Preset' : 'New Preset'}</DialogTitle>
            <DialogDescription>
              {isEditing
                ? 'Modify the preset settings below.'
                : 'Create a new preset with the settings below.'}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            {/* Name */}
            <div className="space-y-1.5">
              <Label htmlFor="preset-name">Preset Name</Label>
              <Input
                id="preset-name"
                value={formData.name}
                onChange={(e) => {
                  setFormData({ ...formData, name: e.target.value });
                  // Clear error when user starts typing
                  if (nameError) setNameError(null);
                }}
                placeholder="my-preset"
                className={nameError ? 'border-destructive' : ''}
                maxLength={MAX_PRESET_NAME_LENGTH}
              />
              {nameError && (
                <p className="text-xs text-destructive">{nameError}</p>
              )}
            </div>

            {/* Source + Comment */}
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label>Source Tag</Label>
                <Input
                  value={formData.source}
                  onChange={(e) => setFormData({ ...formData, source: e.target.value })}
                  placeholder="e.g., tracker name"
                />
              </div>
              <div className="space-y-1.5">
                <Label>Comment</Label>
                <Input
                  value={formData.comment}
                  onChange={(e) => setFormData({ ...formData, comment: e.target.value })}
                  placeholder="Optional"
                />
              </div>
            </div>

            {/* Boolean toggles */}
            <div className="flex flex-wrap gap-4">
              <div className="flex items-center gap-2">
                <Switch
                  id="preset-private"
                  checked={formData.isPrivate}
                  onCheckedChange={(checked) => setFormData({ ...formData, isPrivate: checked })}
                />
                <Label htmlFor="preset-private" className="text-sm">Private</Label>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  id="preset-noDate"
                  checked={formData.noDate}
                  onCheckedChange={(checked) => setFormData({ ...formData, noDate: checked })}
                />
                <Label htmlFor="preset-noDate" className="text-sm">No Date</Label>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  id="preset-noCreator"
                  checked={formData.noCreator}
                  onCheckedChange={(checked) => setFormData({ ...formData, noCreator: checked })}
                />
                <Label htmlFor="preset-noCreator" className="text-sm">No Creator</Label>
              </div>
            </div>

            {/* Trackers */}
            <div className="space-y-1.5">
              <Label>Trackers</Label>
              <div className="space-y-2">
                {formData.trackers.map((tracker, index) => (
                  <div key={index} className="flex gap-2">
                    <Input
                      value={tracker}
                      onChange={(e) => updateTracker(index, e.target.value)}
                      placeholder="https://tracker.example.com/announce"
                      className="flex-1"
                    />
                    {formData.trackers.length > 1 && (
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

            {/* Advanced Options */}
            <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
              <CollapsibleTrigger asChild>
                <Button variant="ghost" size="sm" className="w-full justify-between">
                  Advanced Options
                  <ChevronDown className={`h-4 w-4 transition-transform ${advancedOpen ? 'rotate-180' : ''}`} />
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent className="space-y-4 pt-2">
                <div className="flex flex-wrap gap-4">
                  <div className="flex items-center gap-2">
                    <Switch
                      id="preset-entropy"
                      checked={formData.entropy}
                      onCheckedChange={(checked) => setFormData({ ...formData, entropy: checked })}
                    />
                    <Label htmlFor="preset-entropy" className="text-sm">Add Entropy</Label>
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      id="preset-skipPrefix"
                      checked={formData.skipPrefix}
                      onCheckedChange={(checked) => setFormData({ ...formData, skipPrefix: checked })}
                    />
                    <Label htmlFor="preset-skipPrefix" className="text-sm">Skip Prefix</Label>
                  </div>
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="preset-workers">Workers Override</Label>
                  <Input
                    id="preset-workers"
                    type="number"
                    min={0}
                    max={32}
                    value={formData.workers}
                    onChange={(e) => setFormData({ ...formData, workers: parseInt(e.target.value) || 0 })}
                    placeholder="0 = use default"
                    className="w-32"
                  />
                  <p className="text-xs text-muted-foreground">
                    Override the default workers setting for this preset.
                    Set to 0 to use the default ({defaultWorkers} workers).
                  </p>
                </div>
              </CollapsibleContent>
            </Collapsible>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setEditDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSavePreset} disabled={isSaving || !formData.name.trim()}>
              {isSaving ? 'Saving...' : isEditing ? 'Save Changes' : 'Create Preset'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Preset</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the preset "{presetToDelete}"? This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDeleteConfirm}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
