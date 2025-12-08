import { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Progress } from '@/components/ui/progress';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { FolderOpen, File, Loader2, CheckCircle, XCircle } from 'lucide-react';
import { SelectTorrentFile, SelectPath, SelectFile, VerifyTorrent } from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import { main } from '../../wailsjs/go/models';

type VerifyRequest = main.VerifyRequest;
type VerifyResult = main.VerifyResult;

interface ProgressEvent {
  completed: number;
  total: number;
  hashRate: number;
  percent: number;
}

export function CheckPage() {
  const [torrentPath, setTorrentPath] = useState('');
  const [contentPath, setContentPath] = useState('');
  const [isVerifying, setIsVerifying] = useState(false);
  const [progress, setProgress] = useState<ProgressEvent | null>(null);
  const [result, setResult] = useState<VerifyResult | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    const cancel = EventsOn('verify:progress', (data: ProgressEvent) => {
      setProgress(data);
    });
    return () => {
      EventsOff('verify:progress');
    };
  }, []);

  const handleSelectTorrent = async () => {
    try {
      const path = await SelectTorrentFile();
      if (path) {
        setTorrentPath(path);
      }
    } catch (e) {
      setError(String(e));
    }
  };

  const handleSelectContentFolder = async () => {
    try {
      const path = await SelectPath();
      if (path) {
        setContentPath(path);
      }
    } catch (e) {
      setError(String(e));
    }
  };

  const handleSelectContentFile = async () => {
    try {
      const path = await SelectFile();
      if (path) {
        setContentPath(path);
      }
    } catch (e) {
      setError(String(e));
    }
  };

  const handleVerify = async () => {
    if (!torrentPath || !contentPath) {
      setError('Please select both a torrent file and content path');
      return;
    }

    setError('');
    setResult(null);
    setProgress(null);
    setIsVerifying(true);

    try {
      const req: VerifyRequest = {
        torrentPath,
        contentPath,
      };

      const res = await VerifyTorrent(req);
      setResult(res as VerifyResult);
    } catch (e) {
      setError(String(e));
    } finally {
      setIsVerifying(false);
      setProgress(null);
    }
  };

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-auto p-6 space-y-4">
        <div>
          <h1 className="text-2xl font-semibold">Check Torrent</h1>
          <p className="text-sm text-muted-foreground">Verify torrent data integrity against local files</p>
        </div>

        {/* Main Form Card */}
        <Card>
          <CardContent className="space-y-4">
            {/* Torrent File */}
            <div className="space-y-1.5">
              <Label>Torrent File</Label>
              <div className="flex gap-2">
                <Input
                  value={torrentPath}
                  onChange={(e) => setTorrentPath(e.target.value)}
                  placeholder="Select a .torrent file"
                  className="flex-1"
                />
                <Button variant="outline" onClick={handleSelectTorrent}>
                  <FolderOpen className="h-4 w-4" />
                </Button>
              </div>
            </div>

            {/* Content Path */}
            <div className="space-y-1.5">
              <Label>Content Path</Label>
              <div className="flex gap-2">
                <Input
                  value={contentPath}
                  onChange={(e) => setContentPath(e.target.value)}
                  placeholder="Select the content folder or file"
                  className="flex-1"
                />
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="outline" size="icon" onClick={handleSelectContentFile}>
                      <File className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Select File</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="outline" size="icon" onClick={handleSelectContentFolder}>
                      <FolderOpen className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Select Folder</TooltipContent>
                </Tooltip>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Error */}
        {error && (
          <Card className="border-destructive">
            <CardContent className="py-3">
              <p className="text-destructive text-sm">{error}</p>
            </CardContent>
          </Card>
        )}

        {/* Progress */}
        {isVerifying && progress && (
          <Card>
            <CardContent className="py-4 space-y-3">
              <div className="flex justify-between text-sm">
                <span className="font-medium">Verifying</span>
                <span className="text-muted-foreground">{progress.percent.toFixed(0)}%</span>
              </div>
              <Progress value={progress.percent} />
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>{progress.completed} / {progress.total} pieces</span>
                <span>{progress.hashRate.toFixed(2)} MB/s</span>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Result */}
        {result && (
          <Card className={result.completion >= 100 ? 'border-emerald-500' : 'border-destructive'}>
            <CardContent className="py-4 space-y-3">
              <div className="flex items-center gap-2">
                {result.completion >= 100 ? (
                  <>
                    <CheckCircle className="h-5 w-5 text-emerald-500" />
                    <span className="font-medium text-emerald-600">Verification Passed</span>
                  </>
                ) : (
                  <>
                    <XCircle className="h-5 w-5 text-destructive" />
                    <span className="font-medium text-destructive">Verification Failed</span>
                  </>
                )}
              </div>

              <div className="grid grid-cols-[120px_1fr] gap-x-2 gap-y-1 text-sm">
                <span className="text-muted-foreground">Completed</span>
                <span>
                  {result.goodPieces} / {result.totalPieces} pieces ({result.completion.toFixed(1)}%)
                </span>

                {result.badPieces > 0 && (
                  <>
                    <span className="text-muted-foreground">Bad Pieces</span>
                    <span className="text-destructive">{result.badPieces}</span>
                  </>
                )}
              </div>

              {result.missingFiles && result.missingFiles.length > 0 && (
                <div className="space-y-1">
                  <p className="text-sm font-medium text-destructive">Missing Files:</p>
                  <ul className="text-xs font-mono space-y-0.5 max-h-24 overflow-auto">
                    {result.missingFiles.slice(0, 10).map((file, i) => (
                      <li key={i} className="truncate text-muted-foreground">
                        {file}
                      </li>
                    ))}
                    {result.missingFiles.length > 10 && (
                      <li className="text-muted-foreground">
                        ... and {result.missingFiles.length - 10} more
                      </li>
                    )}
                  </ul>
                </div>
              )}
            </CardContent>
          </Card>
        )}
      </div>

      <div className="bg-background p-4 flex justify-end">
        <Button onClick={handleVerify} disabled={isVerifying || !torrentPath || !contentPath}>
          {isVerifying && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {isVerifying ? 'Verifying...' : 'Verify Torrent'}
        </Button>
      </div>
    </div>
  );
}
