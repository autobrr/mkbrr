package torrent

import (
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/autobrr/go-torrent/metainfo"
	"golang.org/x/text/unicode/norm"
)

// decomposedNames counts the names a torrent stores in decomposed (NFD) form.
// Those are the names a byte-exact client will fail to find when the content
// itself is stored precomposed, as it is nearly everywhere but macOS.
func decomposedNames(info *metainfo.Info) int {
	count := 0
	if !norm.NFC.IsNormalString(info.Name) {
		count++
	}
	for _, f := range info.Files {
		for _, component := range f.Path {
			if !norm.NFC.IsNormalString(component) {
				count++
				break
			}
		}
	}
	return count
}

// nfcPath returns rel in precomposed (NFC) form when that form names the very
// same file on disk, and rel unchanged otherwise.
//
// macOS reports decomposed (NFD) filenames for content on a network mount even
// when the server stores precomposed bytes, so a torrent built from those names
// lists paths that exist nowhere for a byte-exact client (issue #182). Path
// lookup on macOS ignores the difference, so the precomposed form resolves to
// the same file there; on a filesystem that genuinely holds decomposed bytes it
// does not, and we keep the on-disk name so the creator can still seed.
func nfcPath(dir, rel string) string {
	nfc := norm.NFC.String(rel)
	if nfc == rel {
		return rel
	}

	// NFC also rewrites canonical singletons (U+212B ANGSTROM SIGN, the CJK
	// compatibility ideographs) and composition exclusions, which are not
	// decomposition artifacts. Rewriting those would invent a name no
	// filesystem holds, so only accept differences that come from composing
	// a base character with its combining marks.
	for _, r := range rel {
		if r >= utf8.RuneSelf && norm.NFC.String(string(r)) != string(r) {
			return rel
		}
	}

	onDisk, err := os.Lstat(filepath.Join(dir, rel))
	if err != nil {
		return rel
	}
	composed, err := os.Lstat(filepath.Join(dir, nfc))
	if err != nil || !os.SameFile(onDisk, composed) {
		return rel
	}
	return nfc
}

// pathKey normalizes a path for comparison only. Content is opened by its real
// on-disk name; this is purely so a torrent written in one normalization form
// still matches files stored in the other.
func pathKey(rel string) string {
	return norm.NFC.String(filepath.ToSlash(rel))
}

// resolveNormalized returns the real name in dir whose normalized form matches
// name, for when the torrent and the filesystem disagree on NFC versus NFD.
func resolveNormalized(dir, name string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	want := pathKey(name)
	for _, entry := range entries {
		if pathKey(entry.Name()) == want {
			return entry.Name(), true
		}
	}
	return "", false
}
