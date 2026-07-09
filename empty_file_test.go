package tfclean

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApp_processFile_deletesEmptyFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantDeleted bool
		wantContent string
	}{
		{
			name:        "file becomes empty after removing all blocks",
			content:     "moved {\n  from = module.foo[\"hoge\"]\n  to   = module.foo[\"piyo\"]\n}\n",
			wantDeleted: true,
		},
		{
			name:        "file becomes comments-only after cleaning",
			content:     "# header\nmoved {\n  from = module.foo[\"a\"]\n  to   = module.foo[\"b\"]\n}\n",
			wantDeleted: true,
		},
		{
			name:        "pre-existing whitespace-only file is preserved",
			content:     "   \n\n\t\n",
			wantDeleted: false,
			wantContent: "   \n\n\t\n",
		},
		{
			name:        "pre-existing zero-byte file is preserved",
			content:     "",
			wantDeleted: false,
			wantContent: "",
		},
		{
			name:        "pre-existing comments-only file is preserved",
			content:     "# just a note\n// another note\n",
			wantDeleted: false,
			wantContent: "# just a note\n// another note\n",
		},
		{
			name:        "pre-existing block-comment-only file is preserved",
			content:     "/* nothing\n   to see here */\n",
			wantDeleted: false,
			wantContent: "/* nothing\n   to see here */\n",
		},
		{
			name:        "file with remaining resource is kept",
			content:     "resource \"null_resource\" \"keep\" {}\nmoved {\n  from = module.foo[\"a\"]\n  to   = module.foo[\"b\"]\n}\n",
			wantDeleted: false,
			wantContent: "resource \"null_resource\" \"keep\" {}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.tf")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			app := New(&CLI{Dir: dir})
			if err := app.processFile(path, nil); err != nil {
				t.Fatalf("processFile: %v", err)
			}

			_, err := os.Stat(path)
			if tt.wantDeleted {
				if !os.IsNotExist(err) {
					t.Fatalf("expected file to be deleted, stat err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected file to exist, got err = %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if string(got) != tt.wantContent {
				t.Errorf("content mismatch\n got: %q\nwant: %q", string(got), tt.wantContent)
			}
		})
	}
}
