import { useState, useEffect } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
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
};

function optionsToFormData(name: string, options: PresetOptions): PresetFormData {
  return {
    name,
    source: options.Source || '',
    comment: options.Comment || '',
    isPrivate: options.Private ?? true,
    noDate: options.NoDate ?? false,
    noCreator: options.NoCreator ?? false,
    entropy: options.Entropy ?? false,
    skipPrefix: options.SkipPrefix ?? false,
    trackers: options.Trackers && options.Trackers.length > 0 ? options.Trackers : [''],
    pieceLength: options.PieceLength || 0,
    maxPieceLength: options.MaxPieceLength || 0,
  };
}

function formDataToOptions(data: PresetFormData): PresetOptions {
  const options = new preset.Options();
  options.Private = data.isPrivate;
  options.NoDate = data.noDate;
  options.NoCreator = data.noCreator;
  options.Entropy = data.entropy;
  options.SkipPrefix = data.skipPrefix;
  options.Source = data.source;
  options.Comment = data.comment;
  options.Trackers = data.trackers.filter(t => t.trim() !== '');
  options.PieceLength = data.pieceLength;
  options.MaxPieceLength = data.maxPieceLength;
  options.WebSeeds = [];
  options.ExcludePatterns = [];
  options.IncludePatterns = [];
  options.OutputDir = '';
  options.Version = '';
  options.Workers = 0;
  return options;
}

export function SettingsPage() {
  const [presetPath, setPresetPath] = useState('');
  const [presets, setPresets] = useState<Record<string, PresetOptions>>({});
  const [isLoading, setIsLoading] = useState(true);

  // Dialog states
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [presetToDelete, setPresetToDelete] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [originalName, setOriginalName] = useState<string | null>(null);
  const [formData, setFormData] = useState<PresetFormData>(emptyFormData);
  const [isSaving, setIsSaving] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);

  useEffect(() => {
    loadSettings();
  }, []);

  const loadSettings = async () => {
    setIsLoading(true);
    try {
      const [path, presetMap] = await Promise.all([
        GetPresetFilePath().catch(() => ''),
        GetAllPresets().catch(() => ({})),
      ]);
      setPresetPath(path);
      setPresets(presetMap as Record<string, PresetOptions>);
    } catch (e) {
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
    setEditDialogOpen(true);
  };

  const handleEditPreset = (name: string) => {
    const options = presets[name];
    if (options) {
      setFormData(optionsToFormData(name, options));
      setIsEditing(true);
      setOriginalName(name);
      setAdvancedOpen(false);
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
    } catch (e) {
      toast.error('Failed to delete preset: ' + String(e));
    } finally {
      setDeleteDialogOpen(false);
      setPresetToDelete(null);
    }
  };

  const handleSavePreset = async () => {
    if (!formData.name.trim()) return;

    setIsSaving(true);
    try {
      // If renaming, delete old preset first
      if (isEditing && originalName && originalName !== formData.name) {
        await DeletePreset(originalName);
      }

      const options = formDataToOptions(formData);
      await SavePreset(formData.name, options);
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
          <p className="text-sm text-muted-foreground">Manage presets</p>
        </div>

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
              <div className="space-y-1.5">
                <Label className="text-xs">Available Presets ({presetNames.length})</Label>
                {presetNames.length === 0 ? (
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
                              {opts.Source && <span>Source: {opts.Source}</span>}
                              <span>Private: {opts.Private ? 'Yes' : 'No'}</span>
                              {opts.Trackers && opts.Trackers.length > 0 && (
                                <span>Trackers: {opts.Trackers.length}</span>
                              )}
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
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder="my-preset"
              />
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
