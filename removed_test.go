package tfclean

import (
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
)

func TestApp_cutRemovedBlock(t *testing.T) {
	type fields struct {
		hclParser *hclparse.Parser
		CLI       *CLI
	}
	type args struct {
		data []byte
		from string
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
aaa
removed {
  from = module.foo.hoge
  lifecycle {
    destroy = false
  }
}
bbb
`),
				from: "module.foo.hoge",
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
removed {
  from = module.foo.hoge["aaa"]
  lifecycle {
    destroy = false
  }
}
bbb
`),
				from: "module.foo.hoge[\"aaa\"]",
			},
			want:    []byte("\naaa\nbbb\n"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				hclParser: tt.fields.hclParser,
				CLI:       tt.fields.CLI,
			}
			got, err := app.cutRemovedBlock(tt.args.data, tt.args.from)
			if (err != nil) != tt.wantErr {
				t.Errorf("App.cutRemovedBlock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("App.cutRemovedBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}
