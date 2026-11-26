# mkbrr

mkbrr creates, inspects, checks, and modifies torrent files. The CLI, the GUI, and batch mode all create torrents from the same settings.

## Create settings

**Override**:
A create setting that the user gave explicitly for one run: a CLI flag, a value that the user typed or picked in the GUI create form, or a key in a batch job. A value that the GUI form shows from the preset or a tracker default is not an override. An override always wins. Filter patterns are the exception: override patterns add to the preset patterns.
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

**Origin**:
Where a resolved create setting came from: an override, the preset, a tracker default, or the default. The GUI create form shows the origin of each value.
_Avoid_: source (the source tag is a setting)

**Piece length choice**:
The step that picks the piece length from the resolved create settings, the content size, and the tracker rules. It comes after **Resolve**.
_Avoid_: resolve piece length, piece length calculation

**Piece length bounds**:
The smallest and largest piece length that the **piece length choice** can give for one set of tracker rules and create settings. A tracker cap is the largest the upper bound can be. A user max piece length can only lower it.
_Avoid_: min/max exp, clamp range

**Allowed piece lengths**:
The piece lengths that create accepts for an explicit piece length or max piece length, for one set of tracker rules and create settings. The smallest is the lower piece length bound. The largest is the tracker cap, or 128 MiB if the tracker has no cap.
_Avoid_: valid range, explicit max

**Tracker rule**:
A limit that mkbrr enforces for a tracker: a piece length table, a maximum piece length, or a maximum .torrent size. A tracker rule is not a setting, and an override cannot raise it.
_Avoid_: tracker config, tracker default

**Create plan**:
What create will do, worked out before hashing: the content size after filters, the piece length, the predicted .torrent size, the season pack check, the **output path**, and the notices. The GUI preview shows the create plan.
_Avoid_: preview, estimate

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

## Output

**Create run**:
One create, from the resolved create settings to the written .torrent file and its summary. The create run decides the **output path**, hashes the content, writes the file, and returns the summary. The CLI, the GUI, and batch mode all use the same create run.
_Avoid_: create pipeline, create flow

**Output path**:
The path where a **create run** writes the .torrent file. Without `--output`, it is the torrent name with the **filename prefix** and `.torrent`, in the output folder, or in the current folder when there is no output folder. A relative `--output` goes into the output folder. An absolute `--output` wins over the output folder. An `--output` path gets no filename prefix.
_Avoid_: output file, destination

**Filename prefix**:
The text that mkbrr puts before the output .torrent filename, followed by `_`. In create, it is the tracker domain. In modify, it is the preset name when a preset is given. Otherwise, it is the domain of the first `--tracker` URL, or `modified` when there is no `--tracker`. `skip_prefix` removes it. Batch mode never adds it, because the `output` of a batch job gives the filename.
_Avoid_: tracker prefix, domain prefix
