package tfclean

import (
	"reflect"
	"strings"
	"testing"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func TestApp_applyRemovedDeletion(t *testing.T) {
	type fields struct {
		hclParser *hclparse.Parser
		CLI       *CLI
	}
	type args struct {
		data  []byte
		state string
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
removed {
  from = module.foo.hoge
  lifecycle {
    destroy = false
  }
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
removed {
  from = module.foo.hoge["aaa"]
  lifecycle {
    destroy = false
  }
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
# removed
resource "null_resource" "aaa" {}
removed {
  from = module.foo.hoge["aaa"]
  lifecycle {
    destroy = false
  }
}
resource "null_resource" "bbb" {}
`),
			},
			want:    []byte("\n# removed\nresource \"null_resource\" \"aaa\" {}\nresource \"null_resource\" \"bbb\" {}\n"),
			wantErr: false,
		},
		{
			name:   "",
			fields: fields{},
			args: args{
				data: []byte(`
# removed
resource "null_resource" "aaa" {}
removed {
  from = module.foo.hoge["aaa"]
  lifecycle {
    destroy = false
  }
}
removed {
  from = module.foo.hoge["bbb"]
  lifecycle {
    destroy = false
  }
}
resource "null_resource" "bbb" {}
`),
			},
			want: []byte(`
# removed
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
# removed
resource "null_resource" "aaa" {}

removed {
  from = module.foo.hoge["aaa"]
  lifecycle {
    destroy = false
  }
}

removed {
  from = module.foo.hoge["bbb"]
  lifecycle {
    destroy = false
  }
}

resource "null_resource" "bbb" {}
`),
			},
			want: []byte(`
# removed
resource "null_resource" "aaa" {}

resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name:   "string index followed by attrs",
			fields: fields{},
			args: args{
				data: []byte(`
resource "null_resource" "aaa" {}
removed {
  from = module.foo["hoge"].bar.baz
  lifecycle {
    destroy = false
  }
}
resource "null_resource" "bbb" {}
`),
			},
			want:    []byte("\nresource \"null_resource\" \"aaa\" {}\nresource \"null_resource\" \"bbb\" {}\n"),
			wantErr: false,
		},
		{
			name:   "number index followed by attrs",
			fields: fields{},
			args: args{
				data: []byte(`
resource "null_resource" "aaa" {}
removed {
  from = module.foo[0].bar.baz
  lifecycle {
    destroy = false
  }
}
resource "null_resource" "bbb" {}
`),
			},
			want:    []byte("\nresource \"null_resource\" \"aaa\" {}\nresource \"null_resource\" \"bbb\" {}\n"),
			wantErr: false,
		},
		{
			name:   "resource has not been removed",
			fields: fields{},
			args: args{
				data: []byte(`
resource "time_static" "bbb" {}

removed {
  from = time_static.aaa
}
`),
				state: `
{
  "version": 4,
  "resources": [
    {
      "mode": "managed",
      "type": "time_static",
      "name": "aaa",
      "provider": "provider[\"registry.terraform.io/hashicorp/time\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "day": 13,
            "hour": 13,
            "id": "2026-05-13T13:49:53Z",
            "minute": 49,
            "month": 5,
            "rfc3339": "2026-05-13T13:49:53Z",
            "second": 53,
            "triggers": null,
            "unix": 1778680193,
            "year": 2026
          },
          "sensitive_attributes": [],
          "identity_schema_version": 0
        }
      ]
    }
  ]
}
`,
			},
			want: []byte(`
resource "time_static" "bbb" {}

removed {
  from = time_static.aaa
}
`),
			wantErr: false,
		},
		{
			name:   "resource has been removed",
			fields: fields{},
			args: args{
				data: []byte(`
resource "time_static" "bbb" {}

removed {
  from = time_static.aaa
}
`),
				state: `
{
  "version": 4,
  "resources": []
}
`,
			},
			want: []byte(`
resource "time_static" "bbb" {}

`),
			wantErr: false,
		},
		{
			name:   "module has not been removed",
			fields: fields{},
			args: args{
				data: []byte(`
module "bbb" {
  source = "./modules/time"
}

removed {
  from = module.aaa
}
`),
				state: `
{
  "version": 4,
  "resources": [
    {
      "module": "module.aaa",
      "mode": "managed",
      "type": "time_static",
      "name": "this",
      "provider": "provider[\"registry.terraform.io/hashicorp/time\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "day": 13,
            "hour": 14,
            "id": "2026-05-13T14:13:00Z",
            "minute": 13,
            "month": 5,
            "rfc3339": "2026-05-13T14:13:00Z",
            "second": 0,
            "triggers": null,
            "unix": 1778681580,
            "year": 2026
          },
          "sensitive_attributes": [],
          "identity_schema_version": 0
        }
      ]
    }
  ]
}
`,
			},
			want: []byte(`
module "bbb" {
  source = "./modules/time"
}

removed {
  from = module.aaa
}
`),
			wantErr: false,
		},
		{
			name:   "module has been removed",
			fields: fields{},
			args: args{
				data: []byte(`
module "bbb" {
  source = "./modules/time"
}

removed {
  from = module.aaa
}
`),
				state: `
{
  "version": 4,
  "resources": []
}
`,
			},
			want: []byte(`
module "bbb" {
  source = "./modules/time"
}

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
			var state *tfstate.TFState
			if tt.args.state != "" {
				var err error
				state, err = tfstate.Read(t.Context(), strings.NewReader(tt.args.state))
				if err != nil {
					t.Fatal(err)
				}
			}
			got, err := app.applyAllDeletions(tt.args.data, state)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyAllDeletions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyAllDeletions() = %v, want %v", got, tt.want)
			}
		})
	}
}
