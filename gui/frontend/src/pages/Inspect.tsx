import React, { useState, useEffect } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Input } from '@/components/ui/input';
import { FileSearch, FolderOpen, File, Folder, Loader2, ChevronDown, ChevronRight, Lock, Globe, Copy, Check, RotateCcw, Search, X, ChevronsUpDown, Tag, User, Calendar } from 'lucide-react';
import { SelectTorrentFile, InspectTorrent } from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';

type InspectResult = main.InspectResult;
type FileInfo = main.FileInfo;
type TrackerTier = main.TrackerTier;

const STORAGE_KEY = 'mkbrr-inspect-state';

function loadInspectState(): InspectResult | null {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      const parsed = JSON.parse(saved);
      return parsed.torrentInfo || null;
    }
    return null;
  } catch (e) {
    console.error('Failed to load inspect state from localStorage:', e);
    return null;
  }
}

function saveInspectState(torrentInfo: InspectResult | null) {
  try {
    if (torrentInfo) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ torrentInfo }));
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  } catch (e) {
    console.error('Failed to save inspect state to localStorage:', e);
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

interface TreeNode {
  name: string;
  path: string;
  size?: number;
  children: Map<string, TreeNode>;
  isFile: boolean;
  fileCount: number;
}

function buildTree(files: FileInfo[]): TreeNode {
  const root: TreeNode = { name: '', path: '', children: new Map(), isFile: false, fileCount: 0 };

  for (const file of files) {
    const parts = file.path.split('/');
    let current = root;
    let currentPath = '';

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      const isLast = i === parts.length - 1;
      currentPath = currentPath ? `${currentPath}/${part}` : part;

      if (!current.children.has(part)) {
        current.children.set(part, {
          name: part,
          path: currentPath,
          children: new Map(),
          isFile: isLast,
          size: isLast ? file.size : undefined,
          fileCount: 0,
        });
      }
      current = current.children.get(part)!;
    }
  }

  // Calculate file counts for folders
  function countFiles(node: TreeNode): number {
    if (node.isFile) return 1;
    let count = 0;
    for (const child of node.children.values()) {
      count += countFiles(child);
    }
    node.fileCount = count;
    return count;
  }
  countFiles(root);

  return root;
}

function getAllFolderPaths(node: TreeNode, paths: Set<string> = new Set()): Set<string> {
  for (const child of node.children.values()) {
    if (!child.isFile) {
      paths.add(child.path);
      getAllFolderPaths(child, paths);
    }
  }
  return paths;
}

function getMatchingPaths(files: FileInfo[], query: string): Set<string> {
  const paths = new Set<string>();
  const lowerQuery = query.toLowerCase();

  for (const file of files) {
    if (file.path.toLowerCase().includes(lowerQuery)) {
      // Add all parent folder paths
      const parts = file.path.split('/');
      let currentPath = '';
      for (let i = 0; i < parts.length - 1; i++) {
        currentPath = currentPath ? `${currentPath}/${parts[i]}` : parts[i];
        paths.add(currentPath);
      }
    }
  }
  return paths;
}

function FileTree({ files }: { files: FileInfo[] }) {
  const [searchQuery, setSearchQuery] = useState('');
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(() => {
    // Default: expand all for small torrents, collapse for large
    if (files.length <= 50) {
      const root = buildTree(files);
      return getAllFolderPaths(root);
    }
    return new Set<string>();
  });

  const root = buildTree(files);
  const allFolderPaths = getAllFolderPaths(root);

  // Filter files based on search
  const filteredFiles = searchQuery
    ? files.filter(f => f.path.toLowerCase().includes(searchQuery.toLowerCase()))
    : files;

  // Auto-expand folders containing matches when searching
  const effectiveExpanded = searchQuery
    ? getMatchingPaths(files, searchQuery)
    : expandedFolders;

  const toggleFolder = (path: string) => {
    if (searchQuery) return; // Don't allow manual toggle when searching
    setExpandedFolders(prev => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  };

  const expandAll = () => {
    setExpandedFolders(new Set(allFolderPaths));
  };

  const collapseAll = () => {
    setExpandedFolders(new Set());
  };

  const filteredRoot = searchQuery ? buildTree(filteredFiles) : root;

  function renderNode(node: TreeNode, depth: number = 0): React.ReactElement[] {
    const entries = Array.from(node.children.entries()).sort(([, a], [, b]) => {
      if (a.isFile !== b.isFile) return a.isFile ? 1 : -1;
      return a.name.localeCompare(b.name);
    });

    return entries.flatMap(([, child]) => {
      const isExpanded = effectiveExpanded.has(child.path);
      const items: React.ReactElement[] = [];

      if (child.isFile) {
        items.push(
          <div
            key={child.path}
            className="flex items-center gap-2 py-1 text-sm hover:bg-muted/50 rounded px-2 -mx-2"
            style={{ paddingLeft: `${depth * 16 + 8}px` }}
          >
            <File className="h-4 w-4 text-muted-foreground flex-shrink-0" />
            <span className="flex-1 truncate">{child.name}</span>
            {child.size !== undefined && (
              <span className="text-muted-foreground text-xs tabular-nums">{formatBytes(child.size)}</span>
            )}
          </div>
        );
      } else {
        items.push(
          <div
            key={child.path}
            className="flex items-center gap-2 py-1 text-sm hover:bg-muted/50 rounded px-2 -mx-2 cursor-pointer select-none"
            style={{ paddingLeft: `${depth * 16 + 8}px` }}
            onClick={() => toggleFolder(child.path)}
          >
            {isExpanded ? (
              <ChevronDown className="h-4 w-4 text-muted-foreground flex-shrink-0" />
            ) : (
              <ChevronRight className="h-4 w-4 text-muted-foreground flex-shrink-0" />
            )}
            <Folder className="h-4 w-4 text-blue-500 flex-shrink-0" />
            <span className="flex-1 truncate">{child.name}</span>
            <span className="text-muted-foreground text-xs">
              {child.fileCount} {child.fileCount === 1 ? 'file' : 'files'}
            </span>
          </div>
        );

        if (isExpanded) {
          items.push(...renderNode(child, depth + 1));
        }
      }

      return items;
    });
  }

  return (
    <div className="space-y-3">
      {/* Search and controls */}
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search files..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-8 pr-8 h-8 text-sm"
          />
          {searchQuery && (
            <button
              onClick={() => setSearchQuery('')}
              className="absolute right-2 top-1/2 -translate-y-1/2 p-0.5 hover:bg-muted rounded"
            >
              <X className="h-3.5 w-3.5 text-muted-foreground" />
            </button>
          )}
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={expandedFolders.size === allFolderPaths.size ? collapseAll : expandAll}
          disabled={!!searchQuery}
          className="h-8 px-2"
          title={expandedFolders.size === allFolderPaths.size ? 'Collapse All' : 'Expand All'}
        >
          <ChevronsUpDown className="h-4 w-4" />
        </Button>
      </div>

      {/* Filter status */}
      {searchQuery && (
        <p className="text-xs text-muted-foreground">
          Showing {filteredFiles.length} of {files.length} files
        </p>
      )}

      {/* File tree */}
      <div className="font-mono text-sm">
        {filteredFiles.length === 0 && searchQuery ? (
          <p className="text-sm text-muted-foreground text-center py-4">No files match your search</p>
        ) : (
          renderNode(filteredRoot)
        )}
      </div>
    </div>
  );
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (e) {
      toast.error('Failed to copy to clipboard: ' + String(e));
    }
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

interface MetadataBadgeProps {
  icon: React.ReactNode;
  label: string;
  value: string;
  copyable?: boolean;
}

function MetadataBadge({ icon, label, value, copyable }: MetadataBadgeProps) {
  return (
    <div className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-md bg-muted/50 text-sm group">
      <span className="text-muted-foreground">{icon}</span>
      <span className="text-muted-foreground text-xs">{label}</span>
      <span className="font-medium">{value}</span>
      {copyable && (
        <div className="opacity-0 group-hover:opacity-100 transition-opacity ml-0.5">
          <CopyButton text={value} />
        </div>
      )}
    </div>
  );
}

function getTierLabel(tier: number): string {
  if (tier === 0) return 'Primary';
  return `Backup ${tier}`;
}

export function InspectPage() {
  const [torrentInfo, setTorrentInfo] = useState<InspectResult | null>(null);
  const [error, setError] = useState<string>('');
  const [isLoading, setIsLoading] = useState(false);
  const [trackersOpen, setTrackersOpen] = useState(true);
  const [filesOpen, setFilesOpen] = useState(true);

  // Load persisted state on mount
  useEffect(() => {
    const savedInfo = loadInspectState();
    if (savedInfo) {
      setTorrentInfo(savedInfo);
    }
  }, []);

  // Save state when torrentInfo changes
  useEffect(() => {
    saveInspectState(torrentInfo);
  }, [torrentInfo]);

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

  const handleReset = () => {
    setTorrentInfo(null);
    setError('');
    saveInspectState(null);
  };

  // Check if we have any metadata to display
  const hasMetadata = torrentInfo?.source || torrentInfo?.createdBy ||
    (torrentInfo?.creationDate && torrentInfo.creationDate > 0) || torrentInfo?.comment;

  return (
    <div className="h-full overflow-auto">
      <div className="p-6 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold">Inspect Torrent</h1>
            <p className="text-sm text-muted-foreground">View detailed information about a torrent file</p>
          </div>
          <div className="flex gap-2">
            {torrentInfo && (
              <Button variant="outline" onClick={handleReset} disabled={isLoading}>
                <RotateCcw className="mr-2 h-4 w-4" />
                Reset
              </Button>
            )}
            <Button onClick={handleSelectTorrent} disabled={isLoading}>
              {isLoading ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <FolderOpen className="mr-2 h-4 w-4" />
              )}
              {isLoading ? 'Loading...' : 'Select Torrent'}
            </Button>
          </div>
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
              {hasMetadata && (
                <div className="px-5 py-3 border-b bg-muted/30 space-y-2">
                  <div className="flex flex-wrap gap-2">
                    {torrentInfo.source && (
                      <MetadataBadge
                        icon={<Tag className="h-3.5 w-3.5" />}
                        label="Source"
                        value={torrentInfo.source}
                        copyable
                      />
                    )}
                    {torrentInfo.createdBy && (
                      <MetadataBadge
                        icon={<User className="h-3.5 w-3.5" />}
                        label="Created by"
                        value={torrentInfo.createdBy}
                      />
                    )}
                    {torrentInfo.creationDate > 0 && (
                      <MetadataBadge
                        icon={<Calendar className="h-3.5 w-3.5" />}
                        label="Date"
                        value={new Date(torrentInfo.creationDate * 1000).toLocaleDateString()}
                      />
                    )}
                  </div>
                  {torrentInfo.comment && (
                    <p className="text-sm text-muted-foreground italic">
                      "{torrentInfo.comment}"
                    </p>
                  )}
                </div>
              )}

              {/* Trackers section */}
              {((torrentInfo.trackerTiers && torrentInfo.trackerTiers.length > 0) ||
                (torrentInfo.trackers && torrentInfo.trackers.length > 0)) && (
                <Collapsible open={trackersOpen} onOpenChange={setTrackersOpen}>
                  <CollapsibleTrigger asChild>
                    <div className="flex items-center justify-between px-5 py-2.5 border-b cursor-pointer hover:bg-muted/50 transition-colors">
                      <span className="text-sm font-medium">
                        Trackers ({torrentInfo.trackerTiers?.reduce((sum, tier) => sum + tier.trackers.length, 0) || torrentInfo.trackers?.length || 0})
                      </span>
                      <ChevronDown className={`h-4 w-4 text-muted-foreground transition-transform ${trackersOpen ? 'rotate-180' : ''}`} />
                    </div>
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <div className="px-5 py-3 border-b space-y-4">
                      {torrentInfo.trackerTiers && torrentInfo.trackerTiers.length > 0 ? (
                        // New tier-based display
                        torrentInfo.trackerTiers.map((tier, tierIndex) => (
                          <div key={tierIndex} className="space-y-1.5">
                            <div className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                              {getTierLabel(tier.tier)}
                            </div>
                            {tier.trackers.map((tracker, i) => (
                              <div key={i} className="flex items-center gap-2 group pl-2">
                                <code className="text-xs font-mono text-muted-foreground break-all flex-1">
                                  {tracker}
                                </code>
                                <div className="opacity-0 group-hover:opacity-100 transition-opacity">
                                  <CopyButton text={tracker} />
                                </div>
                              </div>
                            ))}
                          </div>
                        ))
                      ) : (
                        // Fallback for cached data without tier info
                        <div className="space-y-1.5">
                          {torrentInfo.trackers?.map((tracker, i) => (
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
                      )}
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
                    <div className="px-5 py-3 max-h-[300px] overflow-auto">
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
