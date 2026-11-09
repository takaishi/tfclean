package tfclean

import (
	"testing"
)

func TestApp_applyAllDeletions_leadingComments(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "removes a comment directly above a moved block",
			data: `resource "null_resource" "aaa" {}

# Renamed from old to aaa
moved {
  from = null_resource.old
  to   = null_resource.aaa
}

resource "null_resource" "bbb" {}
`,
			want: `resource "null_resource" "aaa" {}

resource "null_resource" "bbb" {}
`,
		},
		{
			name: "removes several # and // lines above import and removed blocks",
			data: `# Bring the existing instance under management.
// See the migration ticket.
import {
  id = "i-123"
  to = aws_instance.example
}
  # indented comment
removed {
  from = null_resource.old
}
resource "null_resource" "bbb" {}
`,
			want: `resource "null_resource" "bbb" {}
`,
		},
		{
			name: "keeps a comment separated from the block by a blank line",
			data: `resource "null_resource" "aaa" {}

# Section header

moved {
  from = null_resource.old
  to   = null_resource.aaa
}
resource "null_resource" "bbb" {}
`,
			want: `resource "null_resource" "aaa" {}

# Section header

resource "null_resource" "bbb" {}
`,
		},
		{
			name: "stops at a blank line inside a comment group",
			data: `# General notes

# About this move
moved {
  from = null_resource.old
  to   = null_resource.aaa
}
resource "null_resource" "bbb" {}
`,
			want: `# General notes

resource "null_resource" "bbb" {}
`,
		},
		{
			name: "keeps a trailing comment on the previous line of code",
			data: `locals {
  a = 1
} # end of locals
moved {
  from = null_resource.old
  to   = null_resource.aaa
}
resource "null_resource" "bbb" {}
`,
			want: `locals {
  a = 1
} # end of locals
resource "null_resource" "bbb" {}
`,
		},
		{
			name: "keeps a /* */ comment and the line comments above it",
			data: `# kept
/* block comment
   # not a line comment */
moved {
  from = null_resource.old
  to   = null_resource.aaa
}
resource "null_resource" "bbb" {}
`,
			want: `# kept
/* block comment
   # not a line comment */
resource "null_resource" "bbb" {}
`,
		},
		{
			name: "keeps comment-like lines inside a heredoc",
			data: `locals {
  doc = <<EOT
# looks like a comment
EOT
}
moved {
  from = null_resource.old
  to   = null_resource.aaa
}
resource "null_resource" "bbb" {}
`,
			want: `locals {
  doc = <<EOT
# looks like a comment
EOT
}
resource "null_resource" "bbb" {}
`,
		},
		{
			name: "keeps comments of a block that stays",
			data: `# about bbb
resource "null_resource" "bbb" {}
# about the move
moved {
  from = null_resource.old
  to   = null_resource.aaa
}
`,
			want: `# about bbb
resource "null_resource" "bbb" {}
`,
		},
		{
			name: "keeps the comment of a tfclean-ignore block",
			data: `# reason for keeping
# tfclean-ignore
moved {
  from = null_resource.old
  to   = null_resource.aaa
}
`,
			want: `# reason for keeping
# tfclean-ignore
moved {
  from = null_resource.old
  to   = null_resource.aaa
}
`,
		},
		{
			name: "handles CRLF line endings",
			data: "resource \"null_resource\" \"aaa\" {}\r\n\r\n# about the move\r\nmoved {\r\n  from = null_resource.old\r\n  to   = null_resource.aaa\r\n}\r\n\r\nresource \"null_resource\" \"bbb\" {}\r\n",
			want: "resource \"null_resource\" \"aaa\" {}\r\n\r\nresource \"null_resource\" \"bbb\" {}\r\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{}
			got, err := app.applyAllDeletions([]byte(tt.data), nil)
			if err != nil {
				t.Fatalf("applyAllDeletions() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("applyAllDeletions() got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApp_applyAllDeletions_keepComments(t *testing.T) {
	data := `resource "null_resource" "aaa" {}

# Renamed from old to aaa
moved {
  from = null_resource.old
  to   = null_resource.aaa
}

resource "null_resource" "bbb" {}
`
	want := `resource "null_resource" "aaa" {}

# Renamed from old to aaa

resource "null_resource" "bbb" {}
`
	app := &App{CLI: &CLI{KeepComments: true}}
	got, err := app.applyAllDeletions([]byte(data), nil)
	if err != nil {
		t.Fatalf("applyAllDeletions() error = %v", err)
	}
	if string(got) != want {
		t.Errorf("applyAllDeletions() got = %q, want %q", got, want)
	}
}
