export namespace main {

	export class CreateRequest {
	    path: string;
	    name: string;
	    outputPath: string;
	    pieceLengthExp?: number;
	    maxPieceLength?: number;
	    targetPieceCount?: number;
	    comment?: string;
	    source?: string;
	    outputDir?: string;
	    workers?: number;
	    isPrivate?: boolean;
	    noDate?: boolean;
	    noCreator?: boolean;
	    entropy?: boolean;
	    skipPrefix?: boolean;
	    failOnSeasonWarning?: boolean;
	    trackerUrls: string[];
	    webSeeds: string[];
	    excludePatterns: string[];
	    includePatterns: string[];
	    presetName: string;
	    presetFile: string;
	    defaultWorkers: number;

	    static createFrom(source: any = {}) {
	        return new CreateRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.outputPath = source["outputPath"];
	        this.pieceLengthExp = source["pieceLengthExp"];
	        this.maxPieceLength = source["maxPieceLength"];
	        this.targetPieceCount = source["targetPieceCount"];
	        this.comment = source["comment"];
	        this.source = source["source"];
	        this.outputDir = source["outputDir"];
	        this.workers = source["workers"];
	        this.isPrivate = source["isPrivate"];
	        this.noDate = source["noDate"];
	        this.noCreator = source["noCreator"];
	        this.entropy = source["entropy"];
	        this.skipPrefix = source["skipPrefix"];
	        this.failOnSeasonWarning = source["failOnSeasonWarning"];
	        this.trackerUrls = source["trackerUrls"];
	        this.webSeeds = source["webSeeds"];
	        this.excludePatterns = source["excludePatterns"];
	        this.includePatterns = source["includePatterns"];
	        this.presetName = source["presetName"];
	        this.presetFile = source["presetFile"];
	        this.defaultWorkers = source["defaultWorkers"];
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
	export class TrackerTier {
	    tier: number;
	    trackers: string[];

	    static createFrom(source: any = {}) {
	        return new TrackerTier(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tier = source["tier"];
	        this.trackers = source["trackers"];
	    }
	}
	export class InspectResult {
	    name: string;
	    infoHash: string;
	    size: number;
	    pieceLength: number;
	    pieceCount: number;
	    trackers: string[];
	    trackerTiers: TrackerTier[];
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
	        this.trackerTiers = this.convertValues(source["trackerTiers"], TrackerTier);
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
	export class LaunchCommand {
	    command: string;
	    path: string;

	    static createFrom(source: any = {}) {
	        return new LaunchCommand(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.path = source["path"];
	    }
	}
	export class ModifyRequest {
	    torrentPath: string;
	    trackerUrls: string[];
	    webSeeds: string[];
	    name: string;
	    comment?: string;
	    source?: string;
	    isPrivate?: boolean;
	    noPrivate: boolean;
	    noDate: boolean;
	    noCreator: boolean;
	    entropy?: boolean;
	    noEntropy: boolean;
	    outputDir: string;
	    outputPattern: string;
	    skipPrefix: boolean;
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
	        this.name = source["name"];
	        this.comment = source["comment"];
	        this.source = source["source"];
	        this.isPrivate = source["isPrivate"];
	        this.noPrivate = source["noPrivate"];
	        this.noDate = source["noDate"];
	        this.noCreator = source["noCreator"];
	        this.entropy = source["entropy"];
	        this.noEntropy = source["noEntropy"];
	        this.outputDir = source["outputDir"];
	        this.outputPattern = source["outputPattern"];
	        this.skipPrefix = source["skipPrefix"];
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
	export class SeasonPackInfo {
	    isSeasonPack: boolean;
	    isSuspicious: boolean;
	    season: number;
	    maxEpisode: number;
	    videoFileCount: number;
	    missingEpisodes?: number[];

	    static createFrom(source: any = {}) {
	        return new SeasonPackInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isSeasonPack = source["isSeasonPack"];
	        this.isSuspicious = source["isSuspicious"];
	        this.season = source["season"];
	        this.maxEpisode = source["maxEpisode"];
	        this.videoFileCount = source["videoFileCount"];
	        this.missingEpisodes = source["missingEpisodes"];
	    }
	}
	export class TorrentResult {
	    path: string;
	    infoHash: string;
	    size: number;
	    pieceCount: number;
	    fileCount: number;
	    warning?: string;
	    seasonPackInfo?: SeasonPackInfo;

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
	        this.seasonPackInfo = this.convertValues(source["seasonPackInfo"], SeasonPackInfo);
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
	    missingPieces: number;
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
	        this.missingPieces = source["missingPieces"];
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
	    targetPieceCount?: number;
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
	        this.targetPieceCount = source["targetPieceCount"];
	        this.workers = source["workers"];
	    }
	}

}

export namespace torrent {

	export class SeasonPackInfo {
	    episodes: number[];
	    missing_episodes: number[];
	    season: number;
	    max_episode: number;
	    video_file_count: number;
	    is_season_pack: boolean;
	    is_suspicious: boolean;

	    static createFrom(source: any = {}) {
	        return new SeasonPackInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.episodes = source["episodes"];
	        this.missing_episodes = source["missing_episodes"];
	        this.season = source["season"];
	        this.max_episode = source["max_episode"];
	        this.video_file_count = source["video_file_count"];
	        this.is_season_pack = source["is_season_pack"];
	        this.is_suspicious = source["is_suspicious"];
	    }
	}
	export class Notice {
	    warn: boolean;
	    text: string;

	    static createFrom(source: any = {}) {
	        return new Notice(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.warn = source["warn"];
	        this.text = source["text"];
	    }
	}
	export class CreatePlan {
	    content_size: number;
	    piece_length_exponent: number;
	    piece_length_bytes: number;
	    predicted_torrent_size: number;
	    tracker_size_limit: number;
	    notices: Notice[];
	    season_pack?: SeasonPackInfo;

	    static createFrom(source: any = {}) {
	        return new CreatePlan(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content_size = source["content_size"];
	        this.piece_length_exponent = source["piece_length_exponent"];
	        this.piece_length_bytes = source["piece_length_bytes"];
	        this.predicted_torrent_size = source["predicted_torrent_size"];
	        this.tracker_size_limit = source["tracker_size_limit"];
	        this.notices = this.convertValues(source["notices"], Notice);
	        this.season_pack = this.convertValues(source["season_pack"], SeasonPackInfo);
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


}
