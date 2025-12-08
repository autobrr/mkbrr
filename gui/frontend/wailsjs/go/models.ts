export namespace main {
	
	export class CreateRequest {
	    path: string;
	    name: string;
	    trackerUrls: string[];
	    webSeeds: string[];
	    comment: string;
	    source: string;
	    isPrivate?: boolean;
	    pieceLengthExp: number;
	    maxPieceLength: number;
	    outputPath: string;
	    outputDir: string;
	    noDate: boolean;
	    noCreator: boolean;
	    entropy: boolean;
	    skipPrefix: boolean;
	    excludePatterns: string[];
	    includePatterns: string[];
	    presetName: string;
	    presetFile: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.trackerUrls = source["trackerUrls"];
	        this.webSeeds = source["webSeeds"];
	        this.comment = source["comment"];
	        this.source = source["source"];
	        this.isPrivate = source["isPrivate"];
	        this.pieceLengthExp = source["pieceLengthExp"];
	        this.maxPieceLength = source["maxPieceLength"];
	        this.outputPath = source["outputPath"];
	        this.outputDir = source["outputDir"];
	        this.noDate = source["noDate"];
	        this.noCreator = source["noCreator"];
	        this.entropy = source["entropy"];
	        this.skipPrefix = source["skipPrefix"];
	        this.excludePatterns = source["excludePatterns"];
	        this.includePatterns = source["includePatterns"];
	        this.presetName = source["presetName"];
	        this.presetFile = source["presetFile"];
	    }
	}
	export class FileInfo {
	    path: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.size = source["size"];
	    }
	}
	export class InspectResult {
	    name: string;
	    infoHash: string;
	    size: number;
	    pieceLength: number;
	    pieceCount: number;
	    trackers: string[];
	    webSeeds: string[];
	    isPrivate: boolean;
	    source: string;
	    comment: string;
	    createdBy: string;
	    creationDate: number;
	    fileCount: number;
	    files: FileInfo[];
	
	    static createFrom(source: any = {}) {
	        return new InspectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.infoHash = source["infoHash"];
	        this.size = source["size"];
	        this.pieceLength = source["pieceLength"];
	        this.pieceCount = source["pieceCount"];
	        this.trackers = source["trackers"];
	        this.webSeeds = source["webSeeds"];
	        this.isPrivate = source["isPrivate"];
	        this.source = source["source"];
	        this.comment = source["comment"];
	        this.createdBy = source["createdBy"];
	        this.creationDate = source["creationDate"];
	        this.fileCount = source["fileCount"];
	        this.files = this.convertValues(source["files"], FileInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModifyRequest {
	    torrentPath: string;
	    trackerUrls: string[];
	    webSeeds: string[];
	    comment: string;
	    source: string;
	    isPrivate?: boolean;
	    noDate: boolean;
	    noCreator: boolean;
	    entropy: boolean;
	    skipPrefix: boolean;
	    outputDir: string;
	    outputPattern: string;
	    presetName: string;
	    presetFile: string;
	    dryRun: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModifyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.torrentPath = source["torrentPath"];
	        this.trackerUrls = source["trackerUrls"];
	        this.webSeeds = source["webSeeds"];
	        this.comment = source["comment"];
	        this.source = source["source"];
	        this.isPrivate = source["isPrivate"];
	        this.noDate = source["noDate"];
	        this.noCreator = source["noCreator"];
	        this.entropy = source["entropy"];
	        this.skipPrefix = source["skipPrefix"];
	        this.outputDir = source["outputDir"];
	        this.outputPattern = source["outputPattern"];
	        this.presetName = source["presetName"];
	        this.presetFile = source["presetFile"];
	        this.dryRun = source["dryRun"];
	    }
	}
	export class ModifyResult {
	    outputPath: string;
	    wasModified: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModifyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outputPath = source["outputPath"];
	        this.wasModified = source["wasModified"];
	    }
	}
	export class PresetsResult {
	    presets: Record<string, preset.Options>;
	    errors?: string[];
	
	    static createFrom(source: any = {}) {
	        return new PresetsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.presets = this.convertValues(source["presets"], preset.Options, true);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TorrentResult {
	    path: string;
	    infoHash: string;
	    size: number;
	    pieceCount: number;
	    fileCount: number;
	    warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new TorrentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.infoHash = source["infoHash"];
	        this.size = source["size"];
	        this.pieceCount = source["pieceCount"];
	        this.fileCount = source["fileCount"];
	        this.warning = source["warning"];
	    }
	}
	export class TrackerInfo {
	    maxPieceLength: number;
	    maxTorrentSize: number;
	    defaultSource: string;
	    hasCustomRules: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TrackerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxPieceLength = source["maxPieceLength"];
	        this.maxTorrentSize = source["maxTorrentSize"];
	        this.defaultSource = source["defaultSource"];
	        this.hasCustomRules = source["hasCustomRules"];
	    }
	}
	export class VerifyRequest {
	    torrentPath: string;
	    contentPath: string;
	
	    static createFrom(source: any = {}) {
	        return new VerifyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.torrentPath = source["torrentPath"];
	        this.contentPath = source["contentPath"];
	    }
	}
	export class VerifyResult {
	    completion: number;
	    totalPieces: number;
	    goodPieces: number;
	    badPieces: number;
	    missingFiles: string[];
	
	    static createFrom(source: any = {}) {
	        return new VerifyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.completion = source["completion"];
	        this.totalPieces = source["totalPieces"];
	        this.goodPieces = source["goodPieces"];
	        this.badPieces = source["badPieces"];
	        this.missingFiles = source["missingFiles"];
	    }
	}

}

export namespace preset {
	
	export class Options {
	    private?: boolean;
	    noDate?: boolean;
	    noCreator?: boolean;
	    skipPrefix?: boolean;
	    entropy?: boolean;
	    failOnSeasonWarning?: boolean;
	    comment?: string;
	    source?: string;
	    outputDir?: string;
	    trackers?: string[];
	    webSeeds?: string[];
	    excludePatterns?: string[];
	    includePatterns?: string[];
	    pieceLength?: number;
	    maxPieceLength?: number;
	    workers?: number;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.private = source["private"];
	        this.noDate = source["noDate"];
	        this.noCreator = source["noCreator"];
	        this.skipPrefix = source["skipPrefix"];
	        this.entropy = source["entropy"];
	        this.failOnSeasonWarning = source["failOnSeasonWarning"];
	        this.comment = source["comment"];
	        this.source = source["source"];
	        this.outputDir = source["outputDir"];
	        this.trackers = source["trackers"];
	        this.webSeeds = source["webSeeds"];
	        this.excludePatterns = source["excludePatterns"];
	        this.includePatterns = source["includePatterns"];
	        this.pieceLength = source["pieceLength"];
	        this.maxPieceLength = source["maxPieceLength"];
	        this.workers = source["workers"];
	    }
	}

}

