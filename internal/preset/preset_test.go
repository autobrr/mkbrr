package preset

import (
	"os"
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
