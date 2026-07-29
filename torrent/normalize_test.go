package torrent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/autobrr/go-torrent/metainfo"
	"golang.org/x/text/unicode/norm"
)

// Escaped code points on purpose: a literal accented character in Go source can
// be silently recomposed by an editor, which would make these fixtures vacuous.
const (
	nfcFile = "El d\u00EDa de la marmota.mkv"  // día, precomposed
	nfdFile = "El di\u0301a de la marmota.mkv" // día, decomposed
	nfcSub  = "Subt\u00EDtulos"
	nfdSub  = "Subti\u0301tulos"
	nfdRoot = "Oton\u0303o S01" // Otoño, decomposed
)

// writeContentFile creates root/sub/name with deterministic content.
func writeContentFile(t *testing.T, root, sub, name string, size int) string {
	t.Helper()

	dir := filepath.Join(root, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
	data := make([]byte, size)
	for i := range data {
		data[i] = byte((i*7 + 3) % 251)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
	return path
}

// readTorrentInfo loads the info dictionary back off disk.
func readTorrentInfo(t *testing.T, path string) metainfo.Info {
	t.Helper()

	mi, err := metainfo.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load torrent %q: %v", path, err)
	}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		t.Fatalf("unmarshal info from %q: %v", path, err)
	}
	return info
}

// normalizationInsensitiveFS reports whether looking a name up in its NFC form
// finds a file stored in NFD form, as it does on macOS.
func normalizationInsensitiveFS(t *testing.T, dir string) bool {
	t.Helper()

	probe := filepath.Join(dir, "probe-"+nfdFile)
	if err := os.WriteFile(probe, []byte("x"), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	defer os.Remove(probe)

	_, err := os.Lstat(norm.NFC.String(probe))
	return err == nil
}

// TestCreateTorrent_PathsResolveOnDisk pins the core invariant: every path
// mkbrr writes into a torrent must name a file that actually exists, byte for
// byte. macOS returns NFD names for content a network mount stores as NFC, so
// without normalization the torrent lists paths that exist nowhere (#182).
func TestCreateTorrent_PathsResolveOnDisk(t *testing.T) {
	tempDir := t.TempDir()
	contentDir := filepath.Join(tempDir, nfdRoot)
	writeContentFile(t, contentDir, "", nfdFile, 512*1024)
	writeContentFile(t, contentDir, nfdSub, nfdFile, 64*1024)

	insensitive := normalizationInsensitiveFS(t, contentDir)

	pieceLenExp := uint(16)
	torrentPath := filepath.Join(tempDir, "out.torrent")
	if _, err := Create(CreateOptions{
		Path:           contentDir,
		OutputPath:     torrentPath,
		PieceLengthExp: &pieceLenExp,
		NoCreator:      true,
		NoDate:         true,
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	info := readTorrentInfo(t, torrentPath)
	if len(info.Files) != 2 {
		t.Fatalf("expected 2 files in torrent, got %d", len(info.Files))
	}

	for _, f := range info.Files {
		rel := filepath.Join(f.Path...)
		if _, err := os.Lstat(filepath.Join(contentDir, rel)); err != nil {
			t.Errorf("torrent lists %q, which does not exist on disk: %v", rel, err)
		}
		if insensitive && !norm.NFC.IsNormalString(rel) {
			t.Errorf("path %q is not NFC; a byte-exact client cannot find it", rel)
		}
	}

	if _, err := os.Lstat(filepath.Join(tempDir, info.Name)); err != nil {
		t.Errorf("torrent name %q does not exist on disk: %v", info.Name, err)
	}
	if insensitive && !norm.NFC.IsNormalString(info.Name) {
		t.Errorf("torrent name %q is not NFC", info.Name)
	}
}

// TestCreateTorrent_KeepsUnresolvableNames is the other half of the guard: on a
// filesystem that genuinely stores decomposed bytes, the NFC form names no
// file, so rewriting the path would break the creator's own seeding.
func TestCreateTorrent_KeepsUnresolvableNames(t *testing.T) {
	tempDir := t.TempDir()
	contentDir := filepath.Join(tempDir, "pack")
	writeContentFile(t, contentDir, "", nfdFile, 128*1024)

	if normalizationInsensitiveFS(t, contentDir) {
		t.Skip("filesystem resolves NFC and NFD alike; the guard cannot trigger here")
	}

	pieceLenExp := uint(16)
	torrentPath := filepath.Join(tempDir, "out.torrent")
	if _, err := Create(CreateOptions{
		Path:           contentDir,
		OutputPath:     torrentPath,
		PieceLengthExp: &pieceLenExp,
		NoCreator:      true,
		NoDate:         true,
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	info := readTorrentInfo(t, torrentPath)
	if got := filepath.Join(info.Files[0].Path...); got != nfdFile {
		t.Errorf("expected on-disk name %q to be preserved, got %q", nfdFile, got)
	}
}

// TestVerifyData_NormalizationMismatch covers the cross-platform half: a
// torrent written in one normalization form must verify against content whose
// filenames are stored in the other, so `mkbrr check` catches a broken torrent
// instead of confirming it.
func TestVerifyData_NormalizationMismatch(t *testing.T) {
	tests := []struct {
		name        string
		torrentFile string
		torrentSub  string
		contentFile string
		contentSub  string
	}{
		{"NFC torrent, NFD content", nfcFile, nfcSub, nfdFile, nfdSub},
		{"NFD torrent, NFC content", nfdFile, nfdSub, nfcFile, nfcSub},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			source := filepath.Join(tempDir, "source", "pack")
			target := filepath.Join(tempDir, "target", "pack")

			writeContentFile(t, source, "", tt.torrentFile, 512*1024)
			writeContentFile(t, source, tt.torrentSub, tt.torrentFile, 64*1024)
			writeContentFile(t, target, "", tt.contentFile, 512*1024)
			writeContentFile(t, target, tt.contentSub, tt.contentFile, 64*1024)

			pieceLenExp := uint(16)
			torrentPath := filepath.Join(tempDir, "out.torrent")
			if _, err := Create(CreateOptions{
				Path:           source,
				OutputPath:     torrentPath,
				PieceLengthExp: &pieceLenExp,
				NoCreator:      true,
				NoDate:         true,
			}); err != nil {
				t.Fatalf("Create failed: %v", err)
			}

			result, err := VerifyData(VerifyOptions{
				TorrentPath: torrentPath,
				ContentPath: target,
				Quiet:       true,
			})
			if err != nil {
				t.Fatalf("VerifyData failed: %v", err)
			}

			if len(result.MissingFiles) != 0 {
				t.Errorf("expected no missing files, got %v", result.MissingFiles)
			}
			if result.GoodPieces != result.TotalPieces {
				t.Errorf("expected all %d pieces good, got %d", result.TotalPieces, result.GoodPieces)
			}
			if result.Completion != 100.0 {
				t.Errorf("expected 100%% completion, got %.2f", result.Completion)
			}
		})
	}
}

// TestVerifyData_SingleFileNormalizationMismatch covers single-file torrents,
// where the filename lives in info.name rather than info.files.
func TestVerifyData_SingleFileNormalizationMismatch(t *testing.T) {
	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source")
	target := filepath.Join(tempDir, "target")

	sourceFile := writeContentFile(t, source, "", nfcFile, 256*1024)
	writeContentFile(t, target, "", nfdFile, 256*1024)

	pieceLenExp := uint(16)
	torrentPath := filepath.Join(tempDir, "out.torrent")
	if _, err := Create(CreateOptions{
		Path:           sourceFile,
		OutputPath:     torrentPath,
		PieceLengthExp: &pieceLenExp,
		NoCreator:      true,
		NoDate:         true,
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	result, err := VerifyData(VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: target,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("VerifyData failed: %v", err)
	}
	if len(result.MissingFiles) != 0 {
		t.Errorf("expected no missing files, got %v", result.MissingFiles)
	}
	if result.Completion != 100.0 {
		t.Errorf("expected 100%% completion, got %.2f", result.Completion)
	}
}

// TestVerifyData_ExactNameBeatsNormalizedTwin pins the byte-exact preference in
// verification: when a directory holds both spellings of a name, the file the
// torrent actually names must win, and a stale twin in the other normalization
// form must never shadow it.
func TestVerifyData_ExactNameBeatsNormalizedTwin(t *testing.T) {
	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source", "pack")
	target := filepath.Join(tempDir, "target", "pack")

	writeContentFile(t, source, "", nfcFile, 256*1024)
	writeContentFile(t, target, "", nfcFile, 256*1024)

	if normalizationInsensitiveFS(t, target) {
		t.Skip("filesystem cannot hold both spellings of the same name")
	}
	// stale twin: same name in the other normalization form, wrong size
	writeContentFile(t, target, "", nfdFile, 64*1024)

	pieceLenExp := uint(16)
	torrentPath := filepath.Join(tempDir, "out.torrent")
	if _, err := Create(CreateOptions{
		Path:           source,
		OutputPath:     torrentPath,
		PieceLengthExp: &pieceLenExp,
		NoCreator:      true,
		NoDate:         true,
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	result, err := VerifyData(VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: target,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("VerifyData failed: %v", err)
	}
	if len(result.MissingFiles) != 0 {
		t.Errorf("expected the byte-exact file to match, got missing: %v", result.MissingFiles)
	}
	if result.Completion != 100.0 {
		t.Errorf("expected 100%% completion, got %.2f", result.Completion)
	}
}

// TestNFCPath_LeavesNonDecomposedNamesAlone pins the cases where NFC is not a
// harmless no-op. Canonical singletons and composition exclusions would rewrite
// names that were never decomposition artifacts, creating on another platform
// exactly the bug this change fixes.
func TestNFCPath_LeavesNonDecomposedNamesAlone(t *testing.T) {
	names := []struct {
		name string
		// whether plain NFC would rewrite these bytes, so the case cannot go stale
		rewritten bool
	}{
		{"\u212Bngstrom.mkv", true}, // ANGSTROM SIGN -> U+00C5
		{"2\u2126.mkv", true},       // OHM SIGN -> U+03A9
		{"\u212Aelvin.mkv", true},   // KELVIN SIGN -> ASCII K
		{"\uF900.mkv", true},        // CJK compatibility ideograph -> U+8C48
		{"\U0002F800.mkv", true},    // CJK compatibility ideograph supplement
		{"\u0958.mkv", true},        // DEVANAGARI KA WITH NUKTA, composition exclusion
		{"a\u2000b.mkv", true},      // EN QUAD -> U+2002
		{"caf\u00E9.mkv", false},    // already NFC
		{"plain-ascii.mkv", false},  // untouched
	}

	dir := t.TempDir()
	for _, tt := range names {
		if rewritten := norm.NFC.String(tt.name) != tt.name; rewritten != tt.rewritten {
			t.Fatalf("stale fixture %q: NFC rewrites it = %v, expected %v", tt.name, rewritten, tt.rewritten)
		}
		if err := os.WriteFile(filepath.Join(dir, tt.name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %q: %v", tt.name, err)
		}
		if got := nfcPath(dir, tt.name); got != tt.name {
			t.Errorf("nfcPath rewrote %q to %q; only decomposition artifacts may be rewritten", tt.name, got)
		}
	}
}

// TestDecomposedNames counts what `mkbrr check` warns about: names a torrent
// stores decomposed, which is the state the old create path produced silently.
func TestDecomposedNames(t *testing.T) {
	tests := []struct {
		name string
		info metainfo.Info
		want int
	}{
		{"all precomposed", metainfo.Info{Name: nfcFile}, 0},
		{"decomposed name", metainfo.Info{Name: nfdFile}, 1},
		// not NFC, but not a decomposition artifact either: nfcPath preserves
		// these on create, so warning about them would be a false alarm
		{"canonical singleton", metainfo.Info{Name: "\u212Bngstrom.mkv"}, 0}, // ANGSTROM SIGN
		{
			"singleton path component",
			metainfo.Info{Name: "pack", Files: []metainfo.FileInfo{
				{Path: []string{"2\u2126.mkv"}}, // OHM SIGN
			}},
			0,
		},
		{
			"decomposed path component",
			metainfo.Info{Name: "pack", Files: []metainfo.FileInfo{
				{Path: []string{nfdSub, nfcFile}},
				{Path: []string{"ascii.mkv"}},
			}},
			1,
		},
		{
			"name and files both decomposed",
			metainfo.Info{Name: nfdSub, Files: []metainfo.FileInfo{
				{Path: []string{nfdFile}},
				{Path: []string{nfdSub, nfdFile}},
			}},
			3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decomposedNames(&tt.info); got != tt.want {
				t.Errorf("decomposedNames = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestNFCPath_NormalizesDecomposedNames is the positive case, and asserts the
// fixture really is decomposed so the test above cannot pass vacuously.
func TestNFCPath_NormalizesDecomposedNames(t *testing.T) {
	if norm.NFC.IsNormalString(nfdFile) {
		t.Fatalf("fixture %q is not actually decomposed", nfdFile)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, nfdFile), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := nfcPath(dir, nfdFile)
	if normalizationInsensitiveFS(t, dir) {
		if got != nfcFile {
			t.Errorf("expected %q to be normalized to %q, got %q", nfdFile, nfcFile, got)
		}
	} else if got != nfdFile {
		t.Errorf("expected %q to be preserved on a byte-exact filesystem, got %q", nfdFile, got)
	}
}
