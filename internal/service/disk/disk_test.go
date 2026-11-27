package disk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMounts(t *testing.T) {
	got := ParseMounts(" dl=/data/downloads , /mnt/backup ,, =", "/media")
	assert.Equal(t, []Mount{
		{Label: "media", Path: "/media"},
		{Label: "dl", Path: "/data/downloads"},
		{Label: "backup", Path: "/mnt/backup"},
	}, got)
}

func TestUsage(t *testing.T) {
	svc := New([]Mount{{Label: "tmp", Path: t.TempDir()}})
	got, err := svc.Usage(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "tmp", got[0].Label)
	assert.Greater(t, got[0].Total, int64(0))
	assert.GreaterOrEqual(t, got[0].UsedPercent, 0)
	assert.LessOrEqual(t, got[0].UsedPercent, 100)
}

func TestUsage_SkipsBadMountAndErrorsWhenAllBad(t *testing.T) {
	ok := Mount{Label: "ok", Path: t.TempDir()}
	bad := Mount{Label: "bad", Path: "/nonexistent/path/xyz"}

	got, err := New([]Mount{bad, ok}).Usage(context.Background())
	require.NoError(t, err)
	assert.Len(t, got, 1)

	_, err = New([]Mount{bad}).Usage(context.Background())
	assert.Error(t, err)
}
