import React, { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import { FileSearch, FolderOpen, File, Folder, Loader2 } from 'lucide-react';
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
          className="flex items-center gap-2 py-0.5 text-sm"
          style={{ paddingLeft: `${depth * 16}px` }}
        >
          {child.isFile ? (
            <File className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
          ) : (
            <Folder className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
          )}
          <span className="flex-1 truncate">{child.name}</span>
          {child.size !== undefined && (
            <span className="text-muted-foreground text-xs">{formatBytes(child.size)}</span>
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

export function InspectPage() {
  const [torrentInfo, setTorrentInfo] = useState<InspectResult | null>(null);
  const [error, setError] = useState<string>('');
  const [isLoading, setIsLoading] = useState(false);

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
            <CardContent className="flex flex-col items-center justify-center py-12 text-center">
              <FileSearch className="h-12 w-12 text-muted-foreground mb-4" />
              <p className="text-muted-foreground">Select a torrent file to inspect its contents</p>
            </CardContent>
          </Card>
        )}

        {torrentInfo && (
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader className="py-3">
                <CardTitle className="text-base">General</CardTitle>
              </CardHeader>
              <CardContent className="pt-0">
                <div className="grid grid-cols-[100px_1fr] gap-x-2 gap-y-1 text-sm">
                  <span className="text-muted-foreground">Name</span>
                  <span className="font-medium break-all">{torrentInfo.name}</span>

                  <span className="text-muted-foreground">Hash</span>
                  <span className="font-mono text-xs break-all">{torrentInfo.infoHash}</span>

                  <span className="text-muted-foreground">Size</span>
                  <span>{formatBytes(torrentInfo.size)}</span>

                  <span className="text-muted-foreground">Pieces</span>
                  <span>{torrentInfo.pieceCount.toLocaleString()} × {formatBytes(torrentInfo.pieceLength)}</span>

                  <span className="text-muted-foreground">Files</span>
                  <span>{torrentInfo.fileCount}</span>

                  <span className="text-muted-foreground">Private</span>
                  <span>{torrentInfo.isPrivate ? 'Yes' : 'No'}</span>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="py-3">
                <CardTitle className="text-base">Metadata</CardTitle>
              </CardHeader>
              <CardContent className="pt-0">
                <div className="grid grid-cols-[100px_1fr] gap-x-2 gap-y-1 text-sm">
                  {torrentInfo.source && (
                    <>
                      <span className="text-muted-foreground">Source</span>
                      <span>{torrentInfo.source}</span>
                    </>
                  )}

                  {torrentInfo.comment && (
                    <>
                      <span className="text-muted-foreground">Comment</span>
                      <span className="break-all">{torrentInfo.comment}</span>
                    </>
                  )}

                  {torrentInfo.createdBy && (
                    <>
                      <span className="text-muted-foreground">Created By</span>
                      <span>{torrentInfo.createdBy}</span>
                    </>
                  )}

                  {torrentInfo.creationDate > 0 && (
                    <>
                      <span className="text-muted-foreground">Created</span>
                      <span>{new Date(torrentInfo.creationDate * 1000).toLocaleString()}</span>
                    </>
                  )}

                  {(!torrentInfo.source && !torrentInfo.comment && !torrentInfo.createdBy && !torrentInfo.creationDate) && (
                    <span className="text-muted-foreground col-span-2">No metadata available</span>
                  )}
                </div>
              </CardContent>
            </Card>

            {torrentInfo.trackers && torrentInfo.trackers.length > 0 && (
              <Card>
                <CardHeader className="py-3">
                  <CardTitle className="text-base">Trackers ({torrentInfo.trackers.length})</CardTitle>
                </CardHeader>
                <CardContent className="pt-0">
                  <ScrollArea className="h-24">
                    <div className="space-y-0.5">
                      {torrentInfo.trackers.map((tracker, i) => (
                        <p key={i} className="text-sm font-mono break-all">
                          {tracker}
                        </p>
                      ))}
                    </div>
                  </ScrollArea>
                </CardContent>
              </Card>
            )}

            {torrentInfo.files && torrentInfo.files.length > 0 && (
              <Card className={torrentInfo.trackers && torrentInfo.trackers.length > 0 ? '' : 'md:col-span-2'}>
                <CardHeader className="py-3">
                  <CardTitle className="text-base">
                    Files ({torrentInfo.fileCount}) - {formatBytes(torrentInfo.size)}
                  </CardTitle>
                </CardHeader>
                <CardContent className="pt-0">
                  <ScrollArea className="h-40">
                    <FileTree files={torrentInfo.files} />
                  </ScrollArea>
                </CardContent>
              </Card>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
