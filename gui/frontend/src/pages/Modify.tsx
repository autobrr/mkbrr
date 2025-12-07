import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { FolderOpen, Plus, X, Loader2, ChevronDown } from 'lucide-react';
import { SelectTorrentFile, ModifyTorrent } from '../../wailsjs/go/main/App';

import { main } from '../../wailsjs/go/models';

type ModifyRequest = main.ModifyRequest;
type ModifyResult = main.ModifyResult;

export function ModifyPage() {
  const [torrentPath, setTorrentPath] = useState('');
  const [outputDir, setOutputDir] = useState('');
  const [trackers, setTrackers] = useState<string[]>(['']);
  const [setPrivate, setSetPrivate] = useState<boolean | undefined>(undefined);
  const [source, setSource] = useState('');
  const [comment, setComment] = useState('');
  const [noDate, setNoDate] = useState(false);
  const [noCreator, setNoCreator] = useState(false);

  const [isModifying, setIsModifying] = useState(false);
  const [result, setResult] = useState<ModifyResult | null>(null);
  const [error, setError] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);

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

  const handlePrivateChange = (value: string) => {
    if (value === 'unchanged') {
      setSetPrivate(undefined);
    } else {
      setSetPrivate(value === 'private');
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
        webSeeds: [],
        isPrivate: setPrivate,
        source,
        comment,
        noDate,
        noCreator,
        entropy: false,
        skipPrefix: false,
        outputDir,
        outputPattern: '',
        presetName: '',
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
                <Input
                  value={torrentPath}
                  onChange={(e) => setTorrentPath(e.target.value)}
                  placeholder="Select a .torrent file"
                  className="flex-1"
                />
                <Button variant="outline" onClick={handleSelectInput}>
                  <FolderOpen className="h-4 w-4" />
                </Button>
              </div>
            </div>

            {/* Trackers */}
            <div className="space-y-1.5">
              <Label>Add Trackers</Label>
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

            {/* Private Flag */}
            <div className="space-y-1.5">
              <Label>Private Flag</Label>
              <div className="flex gap-4">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="radio"
                    name="private"
                    checked={setPrivate === undefined}
                    onChange={() => handlePrivateChange('unchanged')}
                    className="w-4 h-4"
                  />
                  <span className="text-sm">Unchanged</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="radio"
                    name="private"
                    checked={setPrivate === true}
                    onChange={() => handlePrivateChange('private')}
                    className="w-4 h-4"
                  />
                  <span className="text-sm">Private</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="radio"
                    name="private"
                    checked={setPrivate === false}
                    onChange={() => handlePrivateChange('public')}
                    className="w-4 h-4"
                  />
                  <span className="text-sm">Public</span>
                </label>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Advanced Options - Collapsible */}
        <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
          <Card>
            <CollapsibleTrigger asChild>
              <CardHeader className="cursor-pointer hover:bg-muted/50 transition-colors py-3">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base font-medium">Advanced Options</CardTitle>
                  <ChevronDown className={`h-4 w-4 text-muted-foreground transition-transform ${advancedOpen ? 'rotate-180' : ''}`} />
                </div>
              </CardHeader>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <CardContent className="pt-0 space-y-4">
                {/* Output Dir */}
                <div className="space-y-1.5">
                  <Label>Output Directory</Label>
                  <Input
                    value={outputDir}
                    onChange={(e) => setOutputDir(e.target.value)}
                    placeholder="Same as input file"
                  />
                </div>

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
                </div>
              </CardContent>
            </CollapsibleContent>
          </Card>
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
            </CardContent>
          </Card>
        )}
      </div>

      <div className="border-t bg-background p-4 flex justify-end">
        <Button onClick={handleModify} disabled={isModifying || !torrentPath}>
          {isModifying && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {isModifying ? 'Modifying...' : 'Modify Torrent'}
        </Button>
      </div>
    </div>
  );
}
