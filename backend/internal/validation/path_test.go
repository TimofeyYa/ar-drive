package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanFilePath(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"plain", "file.txt", "file.txt", nil},
		{"nested", "dir/sub/file.txt", "dir/sub/file.txt", nil},
		{"dot-segments", "dir/./sub/file.txt", "dir/sub/file.txt", nil},
		{"double-slash", "dir//sub/file.txt", "dir/sub/file.txt", nil},
		{"with-spaces", "dir/my file.txt", "dir/my file.txt", nil},
		{"unicode", "папка/файл.txt", "папка/файл.txt", nil},

		{"empty", "", "", ErrEmptyPath},
		{"only-dot", ".", "", ErrEmptyPath},
		{"traversal", "../etc/passwd", "", ErrPathTraversal},
		{"nested-traversal", "dir/../../etc", "", ErrPathTraversal},
		{"absolute", "/etc/passwd", "", ErrLeadingSlash},
		{"backslash", `dir\sub\file`, "", ErrBackslash},
		{"null-byte", "file\x00.txt", "", ErrNullByte},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CleanFilePath(tc.in)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCleanFolderPath(t *testing.T) {
	out, err := CleanFolderPath("dir/sub")
	require.NoError(t, err)
	assert.Equal(t, "dir/sub/", out)

	out, err = CleanFolderPath("dir/sub/")
	require.NoError(t, err)
	assert.Equal(t, "dir/sub/", out)

	_, err = CleanFolderPath("..")
	require.ErrorIs(t, err, ErrPathTraversal)
}
