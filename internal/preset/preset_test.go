package preset

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOutputDirMerging(t *testing.T) {
	// Create a temporary file for test config
	tmpFile, err := os.CreateTemp("", "presets-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write test presets config
	testConfig := `version: 1
default:
  output_dir: "/default/output/dir"
  private: true

presets:
  with_output_dir:
    output_dir: "/preset/output/dir"
    source: "TEST"

  without_output_dir:
    source: "TEST2"
`
	if err := os.WriteFile(tmpFile.Name(), []byte(testConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config
	config, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	// Test 1: Preset with its own output_dir should override default
	presetWithDir, err := config.GetPreset("with_output_dir")
	if err != nil {
		t.Fatalf("Failed to get preset: %v", err)
	}

	if presetWithDir.OutputDir != "/preset/output/dir" {
		t.Errorf("Expected preset output_dir to be '/preset/output/dir', got '%s'", presetWithDir.OutputDir)
	}

	// Test 2: Preset without output_dir should inherit from default
	presetWithoutDir, err := config.GetPreset("without_output_dir")
	if err != nil {
		t.Fatalf("Failed to get preset: %v", err)
	}

	if presetWithoutDir.OutputDir != "/default/output/dir" {
		t.Errorf("Expected preset to inherit default output_dir '/default/output/dir', got '%s'", presetWithoutDir.OutputDir)
	}
}

func TestPresetTargetPieceCountMerge(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "presets-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	tests := []struct {
		name             string
		config           string
		presetName       string
		wantPieceLength  uint
		wantTargetCount  uint
	}{
		{
			name: "preset with both values: last writer wins (target_piece_count clears piece_length)",
			config: `version: 1
presets:
  both:
    piece_length: 20
    target_piece_count: 1000
    source: "TEST"
`,
			presetName:      "both",
			wantPieceLength: 0,
			wantTargetCount: 1000,
		},
		{
			name: "default piece_length overridden by preset target_piece_count",
			config: `version: 1
default:
  piece_length: 20
presets:
  with_target:
    target_piece_count: 1000
    source: "TEST"
`,
			presetName:      "with_target",
			wantPieceLength: 0,
			wantTargetCount: 1000,
		},
		{
			name: "default target_piece_count overridden by preset piece_length",
			config: `version: 1
default:
  target_piece_count: 500
presets:
  with_piece:
    piece_length: 22
    source: "TEST"
`,
			presetName:      "with_piece",
			wantPieceLength: 22,
			wantTargetCount: 0,
		},
		{
			name: "preset target_piece_count alone",
			config: `version: 1
presets:
  target_only:
    target_piece_count: 1000
    source: "TEST"
`,
			presetName:      "target_only",
			wantPieceLength: 0,
			wantTargetCount: 1000,
		},
		{
			name: "preset piece_length alone",
			config: `version: 1
presets:
  piece_only:
    piece_length: 20
    source: "TEST"
`,
			presetName:      "piece_only",
			wantPieceLength: 20,
			wantTargetCount: 0,
		},
		{
			name: "preset target_piece_count overrides default target_piece_count",
			config: `version: 1
default:
  target_piece_count: 500
presets:
  override:
    target_piece_count: 1000
    source: "TEST"
`,
			presetName:      "override",
			wantPieceLength: 0,
			wantTargetCount: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(tmpFile.Name(), []byte(tt.config), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			config, err := Load(tmpFile.Name())
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			preset, err := config.GetPreset(tt.presetName)
			if err != nil {
				t.Fatalf("GetPreset failed: %v", err)
			}

			if preset.PieceLength != tt.wantPieceLength {
				t.Errorf("PieceLength = %d, want %d", preset.PieceLength, tt.wantPieceLength)
			}
			if preset.TargetPieceCount != tt.wantTargetCount {
				t.Errorf("TargetPieceCount = %d, want %d", preset.TargetPieceCount, tt.wantTargetCount)
			}
		})
	}
}

func TestSaveRestrictsPresetFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not meaningful on Windows")
	}

	configPath := filepath.Join(t.TempDir(), "mkbrr", "presets.yaml")
	config := &Config{
		Version: 1,
		Presets: map[string]Options{
			"private-tracker": {
				Trackers: []string{"https://tracker.example/announce?passkey=secret"},
			},
		},
	}

	if err := Save(configPath, config); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	fileInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat saved preset file: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("preset file mode = %o, want 600", got)
	}

	dirInfo, err := os.Stat(filepath.Dir(configPath))
	if err != nil {
		t.Fatalf("stat preset dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("preset dir mode = %o, want 700", got)
	}
}

func TestSaveFixesExistingPresetFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not meaningful on Windows")
	}

	configDir := filepath.Join(t.TempDir(), "mkbrr")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.Chmod(configDir, 0o755); err != nil {
		t.Fatalf("chmod config dir: %v", err)
	}

	configPath := filepath.Join(configDir, "presets.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}
	if err := os.Chmod(configPath, 0o644); err != nil {
		t.Fatalf("chmod existing config: %v", err)
	}

	if err := Save(configPath, &Config{Version: 1, Presets: map[string]Options{}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	fileInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat saved preset file: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("preset file mode = %o, want 600", got)
	}

	dirInfo, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("stat preset dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("preset dir mode = %o, want 700", got)
	}
}
