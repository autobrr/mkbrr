// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"fmt"

	"github.com/autobrr/mkbrr/internal/preset"
)

type ModifyAction string

const (
	ModifyKeep   ModifyAction = "keep"
	ModifySet    ModifyAction = "set"
	ModifyClear  ModifyAction = "clear"
	ModifyRemove ModifyAction = "remove"
)

const OriginKeep Origin = "keep"

type ModifyChange struct {
	Action ModifyAction `json:"action"`
	Origin Origin       `json:"origin"`
}

type ModifyListChange struct {
	Action ModifyAction `json:"action"`
	Origin Origin       `json:"origin"`
	Value  []string     `json:"value"`
}

type ModifyTextChange struct {
	Action ModifyAction `json:"action"`
	Origin Origin       `json:"origin"`
	Value  string       `json:"value"`
}

type ModifyFlagChange struct {
	Action ModifyAction `json:"action"`
	Origin Origin       `json:"origin"`
	Value  bool         `json:"value"`
}

type ModifyResolution struct {
	Trackers     ModifyListChange `json:"trackers"`
	WebSeeds     ModifyListChange `json:"webSeeds"`
	Name         ModifyTextChange `json:"name"`
	Comment      ModifyTextChange `json:"comment"`
	Source       ModifyTextChange `json:"source"`
	Private      ModifyFlagChange `json:"private"`
	CreatedBy    ModifyTextChange `json:"createdBy"`
	CreationDate ModifyChange     `json:"creationDate"`
	Entropy      ModifyChange     `json:"entropy"`
	OutputDir    ModifyTextChange `json:"outputDir"`
	SkipPrefix   ModifyFlagChange `json:"skipPrefix"`
}

func ResolveModify(opts ModifyOptions) (ModifyResolution, error) {
	if err := opts.validate(); err != nil {
		return ModifyResolution{}, err
	}
	p, err := loadModifyPreset(opts)
	if err != nil {
		return ModifyResolution{}, err
	}
	return resolveModify(opts, p), nil
}

func loadModifyPreset(opts ModifyOptions) (*preset.Options, error) {
	if opts.PresetName == "" {
		return &preset.Options{}, nil
	}
	presetPath, err := preset.FindPresetFile(opts.PresetFile)
	if err != nil {
		return nil, fmt.Errorf("could not find preset file: %w", err)
	}
	presets, err := preset.Load(presetPath)
	if err != nil {
		return nil, fmt.Errorf("could not load presets: %w", err)
	}
	p, err := presets.PresetValues(opts.PresetName)
	if err != nil {
		return nil, fmt.Errorf("could not get preset: %w", err)
	}
	return p, nil
}

func resolveModify(opts ModifyOptions, p *preset.Options) ModifyResolution {
	if p == nil {
		p = &preset.Options{}
	}
	var r ModifyResolution

	switch {
	case opts.NoTracker:
		r.Trackers = ModifyListChange{Action: ModifyClear, Origin: OriginOverride}
	case len(opts.RemoveTrackers) > 0:
		r.Trackers = ModifyListChange{Action: ModifyRemove, Origin: OriginOverride, Value: opts.RemoveTrackers}
	default:
		r.Trackers = listSetting(opts.TrackerURLs, p.Trackers)
	}

	if opts.NoWebSeed {
		r.WebSeeds = ModifyListChange{Action: ModifyClear, Origin: OriginOverride}
	} else {
		r.WebSeeds = listSetting(opts.WebSeeds, p.WebSeeds)
	}

	r.Comment = textSetting(opts.Comment, p.Comment)
	r.Source = textSetting(opts.Source, p.Source)
	r.Name = valueSetting(opts.Name, "")
	r.OutputDir = valueSetting(opts.OutputDir, p.OutputDir)

	switch {
	case opts.NoPrivate:
		r.Private = ModifyFlagChange{Action: ModifyClear, Origin: OriginOverride}
	case opts.IsPrivate != nil:
		r.Private = ModifyFlagChange{Action: ModifySet, Origin: OriginOverride, Value: *opts.IsPrivate}
	case p.Private != nil:
		r.Private = ModifyFlagChange{Action: ModifySet, Origin: OriginPreset, Value: *p.Private}
	default:
		r.Private = ModifyFlagChange{Action: ModifyKeep, Origin: OriginKeep}
	}

	switch {
	case opts.NoCreator:
		r.CreatedBy = ModifyTextChange{Action: ModifyClear, Origin: OriginOverride}
	case p.NoCreator != nil && *p.NoCreator:
		r.CreatedBy = ModifyTextChange{Action: ModifyClear, Origin: OriginPreset}
	case p.NoCreator != nil:
		r.CreatedBy = ModifyTextChange{Action: ModifySet, Origin: OriginPreset, Value: createdBy(opts.Version)}
	default:
		r.CreatedBy = ModifyTextChange{Action: ModifyKeep, Origin: OriginKeep}
	}

	switch {
	case opts.NoDate:
		r.CreationDate = ModifyChange{Action: ModifyClear, Origin: OriginOverride}
	case p.NoDate != nil && *p.NoDate:
		r.CreationDate = ModifyChange{Action: ModifyClear, Origin: OriginPreset}
	case p.NoDate != nil:
		r.CreationDate = ModifyChange{Action: ModifyKeep, Origin: OriginPreset}
	default:
		r.CreationDate = ModifyChange{Action: ModifyKeep, Origin: OriginKeep}
	}

	switch {
	case opts.NoEntropy:
		r.Entropy = ModifyChange{Action: ModifyClear, Origin: OriginOverride}
	case opts.Entropy != nil && *opts.Entropy:
		r.Entropy = ModifyChange{Action: ModifySet, Origin: OriginOverride}
	case opts.Entropy != nil:
		r.Entropy = ModifyChange{Action: ModifyKeep, Origin: OriginOverride}
	case p.Entropy != nil && *p.Entropy:
		r.Entropy = ModifyChange{Action: ModifySet, Origin: OriginPreset}
	case p.Entropy != nil:
		r.Entropy = ModifyChange{Action: ModifyKeep, Origin: OriginPreset}
	default:
		r.Entropy = ModifyChange{Action: ModifyKeep, Origin: OriginKeep}
	}

	switch {
	case opts.SkipPrefix:
		r.SkipPrefix = ModifyFlagChange{Action: ModifySet, Origin: OriginOverride, Value: true}
	case p.SkipPrefix != nil:
		r.SkipPrefix = ModifyFlagChange{Action: ModifySet, Origin: OriginPreset, Value: *p.SkipPrefix}
	default:
		r.SkipPrefix = ModifyFlagChange{Action: ModifyKeep, Origin: OriginKeep}
	}

	return r
}

func listSetting(override, presetValue []string) ModifyListChange {
	switch {
	case len(override) > 0:
		return ModifyListChange{Action: ModifySet, Origin: OriginOverride, Value: override}
	case len(presetValue) > 0:
		return ModifyListChange{Action: ModifySet, Origin: OriginPreset, Value: presetValue}
	}
	return ModifyListChange{Action: ModifyKeep, Origin: OriginKeep}
}

func textSetting(override *string, presetValue string) ModifyTextChange {
	switch {
	case override != nil && *override == "":
		return ModifyTextChange{Action: ModifyClear, Origin: OriginOverride}
	case override != nil:
		return ModifyTextChange{Action: ModifySet, Origin: OriginOverride, Value: *override}
	}
	return valueSetting("", presetValue)
}

func valueSetting(override, presetValue string) ModifyTextChange {
	switch {
	case override != "":
		return ModifyTextChange{Action: ModifySet, Origin: OriginOverride, Value: override}
	case presetValue != "":
		return ModifyTextChange{Action: ModifySet, Origin: OriginPreset, Value: presetValue}
	}
	return ModifyTextChange{Action: ModifyKeep, Origin: OriginKeep}
}

func (r ModifyResolution) spec() metadataSpec {
	return metadataSpec{
		Trackers:     metadataField(r.Trackers.Action, r.Trackers.Value),
		WebSeeds:     metadataField(r.WebSeeds.Action, r.WebSeeds.Value),
		Comment:      metadataField(r.Comment.Action, r.Comment.Value),
		CreatedBy:    metadataField(r.CreatedBy.Action, r.CreatedBy.Value),
		CreationDate: metadataField[int64](r.CreationDate.Action, 0),
		Name:         metadataField(r.Name.Action, r.Name.Value),
		Private:      metadataField(r.Private.Action, r.Private.Value),
		Source:       metadataField(r.Source.Action, r.Source.Value),
		Entropy:      r.Entropy.Action.fieldAction(),
	}
}

func (r ModifyResolution) wantsDate() bool {
	return r.CreationDate.Action == ModifyKeep && r.CreationDate.Origin == OriginPreset
}

func metadataField[T any](action ModifyAction, value T) field[T] {
	f := field[T]{action: action.fieldAction()}
	if f.action == setField || f.action == removeField {
		f.value = value
	}
	return f
}

func (a ModifyAction) fieldAction() fieldAction {
	switch a {
	case ModifySet:
		return setField
	case ModifyClear:
		return clearField
	case ModifyRemove:
		return removeField
	default:
		return keepField
	}
}
