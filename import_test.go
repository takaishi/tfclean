package tfclean

import (
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
)

func TestApp_applyImportDeletion(t *testing.T) {
	type fields struct {
		hclParser *hclparse.Parser
		CLI       *CLI
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
resource "null_resource" "aaa" {}
import {
  id = "resource_id"
  to = module.foo.hoge
}
resource "null_resource" "bbb" {}
`),
			},
			want:    []byte("\nresource \"null_resource\" \"aaa\" {}\nresource \"null_resource\" \"bbb\" {}\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
resource "null_resource" "aaa" {}
import {
  id = "resource_id"
  to = module.foo["hoge"]
}
resource "null_resource" "bbb" {}
`),
			},
			want:    []byte("\nresource \"null_resource\" \"aaa\" {}\nresource \"null_resource\" \"bbb\" {}\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
resource "null_resource" "aaa" {}
import {
  id = "resource_id"
  to = module.foo.hoge
}
resource "null_resource" "bbb" {}
`),
			},
			want:    []byte("\n# import\nresource \"null_resource\" \"aaa\" {}\nresource \"null_resource\" \"bbb\" {}\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
resource "null_resource" "aaa" {}
import {
  id = "1234567890:default:hoge"
  to = module.foo["hoge"]
}
resource "null_resource" "bbb" {}
`),
			},
			want:    []byte("\n# import\nresource \"null_resource\" \"aaa\" {}\nresource \"null_resource\" \"bbb\" {}\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
resource "null_resource" "aaa" {}
import {
  id = "1234567890:default:hoge"
  to = module.foo["hoge"]
}
import {
  id = "1234567890:default:piyo"
  to = module.foo["piyo"]
}
resource "null_resource" "bbb" {}
`),
			},
			want: []byte(`
# import
resource "null_resource" "aaa" {}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
resource "null_resource" "aaa" {}

import {
  id = "1234567890:default:hoge"
  to = module.foo["hoge"]
}

import {
  id = "1234567890:default:piyo"
  to = module.foo["piyo"]
}


resource "null_resource" "bbb" {}


`),
			},
			want: []byte(`
# import
resource "null_resource" "aaa" {}

resource "null_resource" "bbb" {}


`),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
resource "null_resource" "aaa" {}
import {
  id = "${local.a}-1"
  to = module.foo[0]
}
resource "null_resource" "bbb" {}
`),
			},
			want: []byte(`
# import
resource "null_resource" "aaa" {}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
resource "null_resource" "aaa" {}
import {
  id = "/cloudwatch/log/group/hoge"
  to = module.foo
}
resource "null_resource" "bbb" {}
`),
			},
			want: []byte(`
# import
resource "null_resource" "aaa" {}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name:   "identity-based import",
			fields: fields{},
			args: args{
				data: []byte(`
# import with identity
resource "null_resource" "aaa" {}
import {
  to = aws_instance.example
  identity = {
    Name = "Example"
  }
}
resource "null_resource" "bbb" {}
`),
			},
			want: []byte(`
# import with identity
resource "null_resource" "aaa" {}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name:   "identity-based import with comment",
			fields: fields{},
			args: args{
				data: []byte(`
# import with identity
resource "null_resource" "aaa" {}
import {
  to = aws_instance.example
  # comment
  identity = {
    Name = "Example"
  }
}
resource "null_resource" "bbb" {}
`),
			},
			want: []byte(`
# import with identity
resource "null_resource" "aaa" {}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name:   "multi identity-based import with comment",
			fields: fields{},
			args: args{
				data: []byte(`
# import with identity
resource "null_resource" "aaa" {}
import {
  to = aws_instance.example
  # comment
  identity = {
    Name = "Example"
  }
}
resource "null_resource" "bbb" {}
import {
  to = aws_instance.example2
  # comment
  identity = {
    Name = "Example2"
  }
}
resource "null_resource" "ccc" {}
`),
			},
			want: []byte(`
# import with identity
resource "null_resource" "aaa" {}
resource "null_resource" "bbb" {}
resource "null_resource" "ccc" {}
`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				hclParser: tt.fields.hclParser,
				CLI:       tt.fields.CLI,
			}
			got, err := app.applyAllDeletions(tt.args.data, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyAllDeletions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyAllDeletions() got = %v, want %v", got, tt.want)
			}
		})
	}
}
