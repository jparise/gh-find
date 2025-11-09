package github

import "testing"

func TestParseFileType(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want FileType
	}{
		{
			name: "regular file",
			mode: "100644",
			want: FileTypeFile,
		},
		{
			name: "group-writable file",
			mode: "100664",
			want: FileTypeFile,
		},
		{
			name: "executable file",
			mode: "100755",
			want: FileTypeExecutable,
		},
		{
			name: "symlink",
			mode: "120000",
			want: FileTypeSymlink,
		},
		{
			name: "directory",
			mode: "040000",
			want: FileTypeDirectory,
		},
		{
			name: "submodule",
			mode: "160000",
			want: FileTypeSubmodule,
		},
		{
			name: "unknown mode defaults to file",
			mode: "123456",
			want: FileTypeFile,
		},
		{
			name: "empty mode defaults to file",
			mode: "",
			want: FileTypeFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFileType(tt.mode)
			if got != tt.want {
				t.Errorf("ParseFileType(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}
