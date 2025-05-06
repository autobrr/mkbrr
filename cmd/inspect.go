package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/torrent"
)

// inspectOptions encapsulates command-line flag values for the inspect command
type inspectOptions struct {
	verbose bool
}

var (
	inspectOpts     = inspectOptions{}
	validateTracker string
	outputFormat    string
	cyan            = color.New(color.FgMagenta, color.Bold).SprintFunc()
	label           = color.New(color.Bold, color.FgHiWhite).SprintFunc()
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <torrent-file>",
	Short: "Inspect a torrent file",
	Long: `Inspect a torrent file, showing its metadata and structure.
Optionally, validate the torrent against known tracker rules.`,
	Args:                       cobra.ExactArgs(1),
	RunE:                       runInspect,
	DisableFlagsInUseLine:      true,
	SuggestionsMinimumDistance: 1,
	SilenceUsage:               true,
}

func init() {
	inspectCmd.Flags().SortFlags = false
	inspectCmd.Flags().BoolVarP(&inspectOpts.verbose, "verbose", "v", false, "show all metadata fields")
	inspectCmd.Flags().StringVarP(&validateTracker, "validate-tracker", "T", "", "validate torrent against rules for a tracker URL or preset name")
	inspectCmd.Flags().StringVarP(&outputFormat, "output-format", "f", "text", "output format ('text' or 'json')")
	inspectCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} <torrent-file> [flags]

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

// loadTorrentData reads the torrent file and extracts metainfo, info, and raw bytes
func loadTorrentData(filePath string) (mi *metainfo.MetaInfo, info *metainfo.Info, rawBytes []byte, validationResults []torrent.ValidationResult, validationErr error, err error) {
	rawBytes, err = os.ReadFile(filePath)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error reading file: %w", err)
	}

	mi, err = metainfo.LoadFromFile(filePath)
	if err != nil {
		return nil, nil, rawBytes, nil, nil, fmt.Errorf("error loading torrent: %w", err)
	}

	parsedInfo, err := mi.UnmarshalInfo()
	if err != nil {
		return mi, nil, rawBytes, nil, nil, fmt.Errorf("error parsing info: %w", err)
	}

	if validateTracker != "" {
		var trackerURL string
		presetPath, err := preset.FindPresetFile("") // check if validateTracker is a preset name
		if err == nil {
			presets, err := preset.Load(presetPath)
			if err == nil {
				presetOpts, err := presets.GetPreset(validateTracker)
				if err == nil && len(presetOpts.Trackers) > 0 {
					trackerURL = presetOpts.Trackers[0]
				}
			}
		}

		if trackerURL == "" {
			trackerURL = validateTracker
		}

		validationResults, validationErr = torrent.ValidateAgainstTrackerRules(mi, &parsedInfo, trackerURL, rawBytes)
	}

	if strings.ToLower(outputFormat) == "json" {
		if validationErr != nil {
			if validationResults == nil {
				validationResults = []torrent.ValidationResult{}
			}
			validationResults = append(validationResults, torrent.ValidationResult{
				Rule:    "Validation Process",
				Status:  torrent.ValidationFail,
				Message: fmt.Sprintf("Error during validation: %v", validationErr),
			})
		}

		jsonData, errGen := torrent.GenerateInspectJSON(mi, &parsedInfo, rawBytes, inspectOpts.verbose, validationResults) // Pass validation results
		if errGen != nil {
			errorJSON := fmt.Sprintf(`{"error": "Failed to generate JSON data: %s"}`, errGen.Error())
			fmt.Println(errorJSON)
			return nil, nil, nil, validationResults, validationErr, errGen
		}
		jsonBytes, errMarshal := json.MarshalIndent(jsonData, "", "  ")
		if errMarshal != nil {
			errorJSON := fmt.Sprintf(`{"error": "Failed to marshal JSON data: %s"}`, errMarshal.Error())
			fmt.Println(errorJSON)
			return nil, nil, nil, validationResults, validationErr, errMarshal
		}
		fmt.Println(string(jsonBytes))
		return nil, nil, nil, validationResults, validationErr, nil
	}

	return mi, &parsedInfo, rawBytes, validationResults, validationErr, nil
}

// displayStandardInfo shows the core information about the torrent
func displayStandardInfo(display *torrent.Display, mi *metainfo.MetaInfo, info *metainfo.Info, validationResults []torrent.ValidationResult, validationErr error) {
	t := &torrent.Torrent{MetaInfo: mi}
	display.ShowTorrentInfo(t, info)

	if validateTracker != "" {
		var displayTrackerName string
		var isPreset bool
		presetPath, _ := preset.FindPresetFile("")
		if presetPath != "" {
			presets, _ := preset.Load(presetPath)
			if presets != nil {
				presetOpts, err := presets.GetPreset(validateTracker)
				if err == nil && len(presetOpts.Trackers) > 0 {
					displayTrackerName = presetOpts.Trackers[0]
					isPreset = true
				}
			}
		}
		if !isPreset {
			displayTrackerName = validateTracker
		}

		displayURL := displayTrackerName
		parsedURL, parseErr := url.Parse(displayTrackerName)
		if parseErr == nil {
			displayURL = parsedURL.Scheme + "://" + parsedURL.Host + "/..."
		}

		if isPreset {
			fmt.Printf("\n%s %s (using preset '%s')\n", cyan("Validation Results for:"), displayURL, validateTracker)
		} else {
			fmt.Printf("\n%s %s\n", cyan("Validation Results for:"), displayURL)
		}

		if validationErr != nil {
			display.ShowError(fmt.Sprintf("Validation error: %v", validationErr))
		}

		if len(validationResults) > 0 {
			display.ShowValidationResults(validationResults)
		} else if validationErr == nil {
			fmt.Println("  No validation results generated.")
		}
	}
}

// displayVerboseInfo shows additional metadata fields found in the torrent file
func displayVerboseInfo(rawBytes []byte, mi *metainfo.MetaInfo) {
	fmt.Printf("%s\n", cyan("Additional metadata:"))

	// Display extra root-level fields
	rootMap := make(map[string]interface{})
	if err := bencode.Unmarshal(rawBytes, &rootMap); err == nil {
		standardRoot := map[string]bool{
			"announce": true, "announce-list": true, "comment": true,
			"created by": true, "creation date": true, "info": true,
			"url-list": true, "nodes": true,
		}

		for k, v := range rootMap {
			if !standardRoot[k] {
				fmt.Printf("  %-13s %v\n", label(k+":"), v)
			}
		}
	}

	// Display extra info-dictionary fields
	infoMap := make(map[string]interface{})
	if err := bencode.Unmarshal(mi.InfoBytes, &infoMap); err == nil {
		standardInfo := map[string]bool{
			"name": true, "piece length": true, "pieces": true,
			"files": true, "length": true, "private": true,
			"source": true, "path": true, "paths": true,
			"md5sum": true,
		}

		for k, v := range infoMap {
			if !standardInfo[k] {
				fmt.Printf("  %-13s %v\n", label("info."+k+":"), v)
			}
		}
	}
	fmt.Println()
}

// displayFileTreeIfNeeded shows the file tree if the torrent contains multiple files
func displayFileTreeIfNeeded(display *torrent.Display, info *metainfo.Info) {
	if info.IsDir() {
		display.ShowFileTree(info)
	}
}

func runInspect(cmd *cobra.Command, args []string) error {
	torrentPath := args[0]

	mi, info, rawBytes, validationResults, validationErr, err := loadTorrentData(torrentPath)
	if err != nil {
		return err
	}

	if mi == nil && info == nil && strings.ToLower(outputFormat) == "json" {
		return nil
	}

	display := torrent.NewDisplay(torrent.NewFormatter(inspectOpts.verbose))
	displayStandardInfo(display, mi, info, validationResults, validationErr)

	if inspectOpts.verbose {
		displayVerboseInfo(rawBytes, mi)
		displayFileTreeIfNeeded(display, info)
	}

	return nil
}
