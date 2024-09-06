package tfclean

import (
	"github.com/hashicorp/hcl/v2/hclparse"
	"reflect"
	"testing"
)

func TestApp_cutMovedBlock(t *testing.T) {
	type fields struct {
		hclParser *hclparse.Parser
		CLI       *CLI
	}
	type args struct {
		data []byte
		to   string
		from string
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
aaa
moved {
  from = module.foo.hoge
  to   = module.foo.piyo
}
bbb
`),
				from: "module.foo.hoge",
				to:   "module.foo.piyo",
			},
			want:    []byte("\naaa\nbbb\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
aaa
moved {
  from = module.foo["hoge"]
  to   = module.foo["piyo"]
}
bbb
`),
				from: "module.foo[\"hoge\"]",
				to:   "module.foo[\"piyo\"]",
			},
			want:    []byte("\naaa\nbbb\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# moved
aaa
moved {
  from = module.foo.hoge
  to   = module.foo.piyo
}
bbb
`),
				from: "module.foo.hoge",
				to:   "module.foo.piyo",
			},
			want:    []byte("\n# moved\naaa\nbbb\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
aaa
moved {
  from = module.foo["hoge"]
  to   = module.foo["piyo"]
}
moved {
  from = module.foo["foo"]
  to   = module.foo["bar"]
}
bbb
`),
				from: "module.foo[\"hoge\"]",
				to:   "module.foo[\"piyo\"]",
			},
			want: []byte(`
aaa
moved {
  from = module.foo["foo"]
  to   = module.foo["bar"]
}
bbb
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
			got, err := app.cutMovedBlock(tt.args.data, tt.args.to, tt.args.from)
			if (err != nil) != tt.wantErr {
				t.Errorf("cutMovedBlock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("cutMovedBlock() got = %v, want %v", got, tt.want)
			}
		})
	}
}
