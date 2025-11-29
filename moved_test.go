package tfclean

import (
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
)

func TestApp_applyMovedDeletion(t *testing.T) {
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
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
resource "null_resource" "aaa" {}
moved {
  from = module.foo.hoge
  to   = module.foo.piyo
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
moved {
  from = module.foo["hoge"]
  to   = module.foo["piyo"]
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
# moved
resource "null_resource" "aaa" {}
moved {
  from = module.foo.hoge
  to   = module.foo.piyo
}
resource "null_resource" "bbb" {}
`),
			},
			want:    []byte("\n# moved\nresource \"null_resource\" \"aaa\" {}\nresource \"null_resource\" \"bbb\" {}\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
resource "null_resource" "aaa" {}
moved {
  from = module.foo["hoge"]
  to   = module.foo["piyo"]
}
moved {
  from = module.foo["foo"]
  to   = module.foo["bar"]
}
resource "null_resource" "bbb" {}
`),
			},
			want: []byte(`
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
# moved
resource "null_resource" "aaa" {}

moved {
  from = module.foo["hoge"]
  to   = module.foo["piyo"]
}

moved {
  from = module.foo["foo"]
  to   = module.foo["bar"]
}

resource "null_resource" "bbb" {}
`),
			},
			want: []byte(`
# moved
resource "null_resource" "aaa" {}

resource "null_resource" "bbb" {}
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
