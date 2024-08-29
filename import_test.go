package tfclean

import (
	"github.com/hashicorp/hcl/v2/hclparse"
	"reflect"
	"testing"
)

func TestApp_cutImportBlock(t *testing.T) {
	type fields struct {
		hclParser *hclparse.Parser
		CLI       *CLI
	}
	type args struct {
		data []byte
		to   string
		id   string
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
import {
  id = "resource_id"
  to = module.foo.hoge
}
bbb
`),
				to: "module.foo.hoge",
				id: "resource_id",
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
			got, err := app.cutImportBlock(tt.args.data, tt.args.to, tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("cutImportBlock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("cutImportBlock() got = %v, want %v", got, tt.want)
			}
		})
	}
}
