import React, { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { FileSearch, FolderOpen, File, Folder, Loader2, ChevronDown, Lock, Globe, Copy, Check } from 'lucide-react';
import { SelectTorrentFile, InspectTorrent } from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';

type InspectResult = main.InspectResult;
type FileInfo = main.FileInfo;

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function FileTree({ files }: { files: FileInfo[] }) {
  interface TreeNode {
    name: string;
    size?: number;
    children: Map<string, TreeNode>;
    isFile: boolean;
  }

  const root: TreeNode = { name: '', children: new Map(), isFile: false };

  for (const file of files) {
    const parts = file.path.split('/');
    let current = root;

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      const isLast = i === parts.length - 1;

      if (!current.children.has(part)) {
        current.children.set(part, {
          name: part,
          children: new Map(),
          isFile: isLast,
          size: isLast ? file.size : undefined,
        });
      }
      current = current.children.get(part)!;
    }
  }

  function renderNode(node: TreeNode, depth: number = 0): React.ReactElement[] {
    const entries = Array.from(node.children.entries()).sort(([, a], [, b]) => {
      if (a.isFile !== b.isFile) return a.isFile ? 1 : -1;
      return a.name.localeCompare(b.name);
    });

    return entries.flatMap(([, child]) => {
      const items: React.ReactElement[] = [
        <div
          key={child.name + depth}
          className="flex items-center gap-2 py-1 text-sm hover:bg-muted/50 rounded px-2 -mx-2"
          style={{ paddingLeft: `${depth * 16 + 8}px` }}
        >
          {child.isFile ? (
            <File className="h-4 w-4 text-muted-foreground flex-shrink-0" />
          ) : (
            <Folder className="h-4 w-4 text-blue-500 flex-shrink-0" />
          )}
          <span className="flex-1 truncate">{child.name}</span>
          {child.size !== undefined && (
            <span className="text-muted-foreground text-xs tabular-nums">{formatBytes(child.size)}</span>
          )}
        </div>,
      ];

      if (!child.isFile) {
        items.push(...renderNode(child, depth + 1));
      }

      return items;
    });
  }

  return <div className="font-mono text-sm">{renderNode(root)}</div>;
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <button
      onClick={handleCopy}
      className="p-1 hover:bg-muted rounded transition-colors"
      title="Copy to clipboard"
    >
      {copied ? (
        <Check className="h-3.5 w-3.5 text-emerald-500" />
      ) : (
        <Copy className="h-3.5 w-3.5 text-muted-foreground" />
      )}
    </button>
  );
}

function StatItem({ value, label }: { value: string; label: string }) {
  return (
    <div className="flex flex-col items-center px-4 py-2">
      <span className="text-lg font-semibold tabular-nums">{value}</span>
      <span className="text-xs text-muted-foreground">{label}</span>
    </div>
  );
}

export function InspectPage() {
  const [torrentInfo, setTorrentInfo] = useState<InspectResult | null>(null);
  const [error, setError] = useState<string>('');
  const [isLoading, setIsLoading] = useState(false);
  const [trackersOpen, setTrackersOpen] = useState(true);
  const [filesOpen, setFilesOpen] = useState(true);

  const handleSelectTorrent = async () => {
    try {
      setError('');
      setIsLoading(true);
      const path = await SelectTorrentFile();
      if (path) {
        const info = await InspectTorrent(path);
        setTorrentInfo(info);
      }
    } catch (e) {
      setError(String(e));
      setTorrentInfo(null);
    } finally {
      setIsLoading(false);
    }
  };

  const metadataItems = [];
  if (torrentInfo?.source) metadataItems.push(`Source: ${torrentInfo.source}`);
  if (torrentInfo?.createdBy) metadataItems.push(`Created by ${torrentInfo.createdBy}`);
  if (torrentInfo?.creationDate && torrentInfo.creationDate > 0) {
    metadataItems.push(new Date(torrentInfo.creationDate * 1000).toLocaleDateString());
  }

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-auto p-6 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold">Inspect Torrent</h1>
            <p className="text-sm text-muted-foreground">View detailed information about a torrent file</p>
          </div>
          <Button onClick={handleSelectTorrent} disabled={isLoading}>
            {isLoading ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <FolderOpen className="mr-2 h-4 w-4" />
            )}
            {isLoading ? 'Loading...' : 'Select Torrent'}
          </Button>
        </div>

        {error && (
          <Card className="border-destructive">
            <CardContent className="py-3">
              <p className="text-destructive text-sm">{error}</p>
            </CardContent>
          </Card>
        )}

        {!torrentInfo && !error && (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-16 text-center">
              <FileSearch className="h-12 w-12 text-muted-foreground/50 mb-4" />
              <p className="text-muted-foreground">Select a torrent file to inspect its contents</p>
            </CardContent>
          </Card>
        )}

        {torrentInfo && (
          <Card>
            <CardContent className="p-0">
              {/* Header with name and hash */}
              <div className="p-5 border-b">
                <div className="flex items-start gap-3">
                  <div className="p-2 bg-muted rounded-lg">
                    <File className="h-6 w-6 text-muted-foreground" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <h2 className="text-lg font-semibold truncate" title={torrentInfo.name}>
                      {torrentInfo.name}
                    </h2>
                    <div className="flex items-center gap-1.5 mt-1">
                      <code className="text-xs text-muted-foreground font-mono truncate">
                        {torrentInfo.infoHash}
                      </code>
                      <CopyButton text={torrentInfo.infoHash} />
                    </div>
                  </div>
                </div>
              </div>

              {/* Stats row */}
              <div className="flex items-center justify-center border-b divide-x">
                <StatItem value={formatBytes(torrentInfo.size)} label="Size" />
                <StatItem value={torrentInfo.pieceCount.toLocaleString()} label="Pieces" />
                <StatItem value={formatBytes(torrentInfo.pieceLength)} label="Piece Size" />
                <StatItem value={torrentInfo.fileCount.toString()} label={torrentInfo.fileCount === 1 ? 'File' : 'Files'} />
                <div className="flex flex-col items-center px-4 py-2">
                  {torrentInfo.isPrivate ? (
                    <Lock className="h-5 w-5 text-amber-500" />
                  ) : (
                    <Globe className="h-5 w-5 text-muted-foreground" />
                  )}
                  <span className="text-xs text-muted-foreground mt-1">
                    {torrentInfo.isPrivate ? 'Private' : 'Public'}
                  </span>
                </div>
              </div>

              {/* Metadata row */}
              {(metadataItems.length > 0 || torrentInfo.comment) && (
                <div className="px-5 py-3 border-b bg-muted/30">
                  {metadataItems.length > 0 && (
                    <p className="text-sm text-muted-foreground">
                      {metadataItems.join(' · ')}
                    </p>
                  )}
                  {torrentInfo.comment && (
                    <p className="text-sm text-muted-foreground mt-1 italic">
                      "{torrentInfo.comment}"
                    </p>
                  )}
                </div>
              )}

              {/* Trackers section */}
              {torrentInfo.trackers && torrentInfo.trackers.length > 0 && (
                <Collapsible open={trackersOpen} onOpenChange={setTrackersOpen}>
                  <CollapsibleTrigger asChild>
                    <div className="flex items-center justify-between px-5 py-2.5 border-b cursor-pointer hover:bg-muted/50 transition-colors">
                      <span className="text-sm font-medium">
                        Trackers ({torrentInfo.trackers.length})
                      </span>
                      <ChevronDown className={`h-4 w-4 text-muted-foreground transition-transform ${trackersOpen ? 'rotate-180' : ''}`} />
                    </div>
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <div className="px-5 py-3 border-b space-y-1.5">
                      {torrentInfo.trackers.map((tracker, i) => (
                        <div key={i} className="flex items-center gap-2 group">
                          <code className="text-xs font-mono text-muted-foreground break-all flex-1">
                            {tracker}
                          </code>
                          <div className="opacity-0 group-hover:opacity-100 transition-opacity">
                            <CopyButton text={tracker} />
                          </div>
                        </div>
                      ))}
                    </div>
                  </CollapsibleContent>
                </Collapsible>
              )}

              {/* Files section */}
              {torrentInfo.files && torrentInfo.files.length > 0 && (
                <Collapsible open={filesOpen} onOpenChange={setFilesOpen}>
                  <CollapsibleTrigger asChild>
                    <div className="flex items-center justify-between px-5 py-2.5 cursor-pointer hover:bg-muted/50 transition-colors">
                      <span className="text-sm font-medium">
                        Files ({torrentInfo.fileCount})
                      </span>
                      <ChevronDown className={`h-4 w-4 text-muted-foreground transition-transform ${filesOpen ? 'rotate-180' : ''}`} />
                    </div>
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <div className="px-5 py-3 max-h-64 overflow-auto">
                      <FileTree files={torrentInfo.files} />
                    </div>
                  </CollapsibleContent>
                </Collapsible>
              )}
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}
