package cmd

import (
	"strings"
	"testing"
)

func TestParseRenamePairsNormalizesBeforeDuplicateValidation(t *testing.T) {
	_, err := parseRenamePairs([]string{
		"./old.bin=x.bin",
		"old.bin=y.bin",
	})
	if err == nil {
		t.Fatal("parseRenamePairs() error = nil, want duplicate source error")
	}
	if !strings.Contains(err.Error(), "duplicate rename source") {
		t.Fatalf("parseRenamePairs() error = %q, want duplicate source error", err)
	}
}

func TestParseRenamePairsReturnsCanonicalPathsWithoutTrimmingNames(t *testing.T) {
	renames, err := parseRenamePairs([]string{`./nested\\old.bin=/archive/../new.bin`, ` old.bin= new.bin`})
	if err != nil {
		t.Fatalf("parseRenamePairs() error: %v", err)
	}
	if got, want := renames["nested/old.bin"], "new.bin"; got != want {
		t.Errorf("parseRenamePairs() mapping = %q, want %q", got, want)
	}
	if got, want := renames[" old.bin"], " new.bin"; got != want {
		t.Errorf("parseRenamePairs() whitespace mapping = %q, want %q", got, want)
	}
}
