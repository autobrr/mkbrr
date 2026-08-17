package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRenamePairsNormalizesBeforeDuplicateValidation(t *testing.T) {
	_, err := parseRenamePairs([]string{
		"./old.bin=x.bin",
		"old.bin=y.bin",
	})
	require.ErrorContains(t, err, "duplicate rename source")
}

func TestParseRenamePairsReturnsCanonicalPathsWithoutTrimmingNames(t *testing.T) {
	renames, err := parseRenamePairs([]string{`./nested\\old.bin=archive/../new.bin`, ` old.bin= new.bin`})
	require.NoError(t, err)
	assert.Equal(t, "new.bin", renames["nested/old.bin"])
	assert.Equal(t, " new.bin", renames[" old.bin"])
}

func TestParseRenamePairsRejectsNonRelativePaths(t *testing.T) {
	for _, pair := range []string{
		"../outside.bin=new.bin",
		"/absolute.bin=new.bin",
		"old.bin=///",
		"nested/../../outside.bin=new.bin",
	} {
		t.Run(pair, func(t *testing.T) {
			_, err := parseRenamePairs([]string{pair})
			require.Error(t, err)
		})
	}
}
