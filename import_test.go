package tfclean

import (
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
)

func TestApp_cutImportBlock(t *testing.T) {
	type fields struct {
		hclParser *hclparse.Parser
		CLI       *CLI
	}
	type args struct {
		data         []byte
		to           string
		id           string
		identityHash map[string]string
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
				to:           "module.foo.hoge",
				id:           "resource_id",
				identityHash: nil,
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
import {
  id = "resource_id"
  to = module.foo["hoge"]
}
bbb
`),
				to:           "module.foo[\"hoge\"]",
				id:           "resource_id",
				identityHash: nil,
			},
			want:    []byte("\naaa\nbbb\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
aaa
import {
  id = "resource_id"
  to = module.foo.hoge
}
bbb
`),
				to:           "module.foo.hoge",
				id:           "resource_id",
				identityHash: nil,
			},
			want:    []byte("\n# import\naaa\nbbb\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
aaa
import {
  id = "1234567890:default:hoge"
  to = module.foo["hoge"]
}
bbb
`),
				to:           "module.foo[\"hoge\"]",
				id:           "1234567890:default:hoge",
				identityHash: nil,
			},
			want:    []byte("\n# import\naaa\nbbb\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
aaa
import {
  id = "1234567890:default:hoge"
  to = module.foo["hoge"]
}
import {
  id = "1234567890:default:piyo"
  to = module.foo["piyo"]
}
bbb
`),
				to:           "module.foo[\"piyo\"]",
				id:           "1234567890:default:piyo",
				identityHash: nil,
			},
			want: []byte(`
# import
aaa
import {
  id = "1234567890:default:hoge"
  to = module.foo["hoge"]
}
bbb
`),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
aaa
import {
  id = "${local.a}-1"
  to = module.foo[0]
}
bbb
`),
				to:           "module.foo[0]",
				id:           "${local.a}-1",
				identityHash: nil,
			},
			want: []byte(`
# import
aaa
bbb
`),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# import
aaa
import {
  id = "/cloudwatch/log/group/hoge"
  to = module.foo
}
bbb
`),
				to:           "module.foo",
				id:           "/cloudwatch/log/group/hoge",
				identityHash: nil,
			},
			want: []byte(`
# import
aaa
bbb
`),
			wantErr: false,
		},
		{
			name:   "identity-based import",
			fields: fields{},
			args: args{
				data: []byte(`
# import with identity
aaa
import {
  to = aws_instance.example
  identity = {
    Name = "Example"
  }
}
bbb
`),
				to: "aws_instance.example",
				id: "",
				identityHash: map[string]string{
					"Name": "Example",
				},
			},
			want: []byte(`
# import with identity
aaa
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
			got, err := app.cutImportBlock(tt.args.data, tt.args.to, tt.args.id, tt.args.identityHash)
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
