# mkbrr

mkbrr creates, inspects, checks, and modifies torrent files. The CLI, the GUI, and batch mode all create torrents from the same settings.

## Create settings

**Override**:
A create setting that the user gave explicitly for one run: a CLI flag, a field in the GUI create form, or a key in a batch job. An override always wins. Filter patterns are the exception: override patterns add to the preset patterns.
_Avoid_: flag value, request value

**Preset**:
A named group of create settings in `presets.yaml`. The `default` section fills keys that the named preset leaves out. A preset fills a setting only when no override sets it.
_Avoid_: profile, template

**Tracker default**:
A setting that mkbrr knows for a tracker, such as its source tag. A tracker default applies only when no override and no preset sets that setting.
_Avoid_: tracker rule (a tracker rule is a limit, such as maximum piece length)

**Resolve**:
To merge the overrides, the preset, and the tracker defaults into the final create settings, in that order of priority.
_Avoid_: merge, apply preset

**Piece length choice**:
The step that picks the piece length from the resolved create settings, the content size, and the tracker rules. It comes after **Resolve**.
_Avoid_: resolve piece length, piece length calculation

**Tracker rule**:
A limit that mkbrr enforces for a tracker: a piece length table, a maximum piece length, or a maximum .torrent size. A tracker rule is not a setting, and an override cannot raise it.
_Avoid_: tracker config, tracker default

## Modify settings

**Keep**:
In modify, a setting that no override and no preset sets stays as the existing torrent has it. Modify resolves in this order: override, then preset, then keep. Tracker defaults do not apply to modify.
_Avoid_: unchanged, default

**Set**:
In modify, a setting that an override or a preset gives a value. Modify writes that value to the torrent.
_Avoid_: change, update

**Clear**:
In modify, a setting that an override removes from the torrent, such as `--no-entropy` or `--source ""`. Only an override can clear. A preset cannot clear: a preset `entropy: false` means keep.
_Avoid_: strip, delete
