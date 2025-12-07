import { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { RefreshCw, ExternalLink } from 'lucide-react';
import { GetVersion, GetPresetFilePath, ListPresets } from '../../wailsjs/go/main/App';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';

export function SettingsPage() {
  const [version, setVersion] = useState('');
  const [presetPath, setPresetPath] = useState('');
  const [presets, setPresets] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    loadSettings();
  }, []);

  const loadSettings = async () => {
    setIsLoading(true);
    try {
      const [v, path, presetList] = await Promise.all([
        GetVersion(),
        GetPresetFilePath(),
        ListPresets(),
      ]);
      setVersion(v);
      setPresetPath(path);
      setPresets(presetList);
    } catch (e) {
      console.error('Failed to load settings:', e);
    } finally {
      setIsLoading(false);
    }
  };

  const handleOpenDocs = () => {
    BrowserOpenURL('https://github.com/autobrr/mkbrr');
  };

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-auto p-6 space-y-4">
        <div>
          <h1 className="text-2xl font-semibold">Settings</h1>
          <p className="text-sm text-muted-foreground">Application settings and information</p>
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          <Card>
            <CardHeader className="py-3">
              <CardTitle className="text-base">About</CardTitle>
            </CardHeader>
            <CardContent className="pt-0 space-y-3">
              <div className="grid grid-cols-[80px_1fr] gap-x-2 gap-y-1 text-sm">
                <span className="text-muted-foreground">Version</span>
                <span className="font-mono">{version || 'Loading...'}</span>
                <span className="text-muted-foreground">License</span>
                <span>MIT</span>
              </div>
              <Separator />
              <Button variant="outline" size="sm" onClick={handleOpenDocs}>
                <ExternalLink className="mr-2 h-4 w-4" />
                Documentation
              </Button>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="py-3">
              <CardTitle className="text-base">Presets</CardTitle>
            </CardHeader>
            <CardContent className="pt-0 space-y-3">
              <div className="space-y-1.5">
                <Label className="text-xs">Preset File Location</Label>
                <div className="flex gap-2">
                  <Input
                    value={presetPath}
                    readOnly
                    className="flex-1 font-mono text-xs h-8"
                  />
                  <Button variant="outline" size="sm" onClick={loadSettings}>
                    <RefreshCw className="h-4 w-4" />
                  </Button>
                </div>
              </div>
              <Separator />
              <div className="space-y-1.5">
                <Label className="text-xs">Available Presets ({presets.length})</Label>
                {presets.length === 0 ? (
                  <p className="text-xs text-muted-foreground">
                    No presets configured
                  </p>
                ) : (
                  <ScrollArea className="h-20">
                    <div className="space-y-1">
                      {presets.map((preset) => (
                        <div
                          key={preset}
                          className="rounded bg-muted px-2 py-1 text-xs font-mono"
                        >
                          {preset}
                        </div>
                      ))}
                    </div>
                  </ScrollArea>
                )}
              </div>
            </CardContent>
          </Card>

          <Card className="lg:col-span-2">
            <CardHeader className="py-3">
              <CardTitle className="text-base">Keyboard Shortcuts</CardTitle>
            </CardHeader>
            <CardContent className="pt-0">
              <div className="grid gap-2 sm:grid-cols-3">
                <div className="flex items-center justify-between rounded border px-3 py-2">
                  <span className="text-sm">Create Torrent</span>
                  <kbd className="inline-flex h-5 items-center gap-1 rounded border bg-muted px-1.5 font-mono text-xs">
                    <span className="text-xs">⌘</span>N
                  </kbd>
                </div>
                <div className="flex items-center justify-between rounded border px-3 py-2">
                  <span className="text-sm">Open Torrent</span>
                  <kbd className="inline-flex h-5 items-center gap-1 rounded border bg-muted px-1.5 font-mono text-xs">
                    <span className="text-xs">⌘</span>O
                  </kbd>
                </div>
                <div className="flex items-center justify-between rounded border px-3 py-2">
                  <span className="text-sm">Toggle Theme</span>
                  <kbd className="inline-flex h-5 items-center gap-1 rounded border bg-muted px-1.5 font-mono text-xs">
                    <span className="text-xs">⌘</span>D
                  </kbd>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
