package tfclean

import (
	"reflect"
	"testing"
)

func TestApp_applyAllDeletions_ignoreAnnotations(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    []byte
		wantErr bool
	}{
		{
			name: "tfclean-ignore preserves the import block immediately below it",
			data: []byte(`
resource "null_resource" "aaa" {}
# tfclean-ignore: keep this for a follow-up change
import {
  id = "resource_id"
  to = module.foo.hoge
}
resource "null_resource" "bbb" {}
`),
			want: []byte(`
resource "null_resource" "aaa" {}
# tfclean-ignore: keep this for a follow-up change
import {
  id = "resource_id"
  to = module.foo.hoge
}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name: "tfclean-ignore without a reason still preserves the block",
			data: []byte(`
resource "null_resource" "aaa" {}
# tfclean-ignore
import {
  id = "resource_id"
  to = module.foo.hoge
}
resource "null_resource" "bbb" {}
`),
			want: []byte(`
resource "null_resource" "aaa" {}
# tfclean-ignore
import {
  id = "resource_id"
  to = module.foo.hoge
}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name: "only the annotated block is preserved, others are still removed",
			data: []byte(`
resource "null_resource" "aaa" {}
import {
  id = "1234567890:default:hoge"
  to = module.foo["hoge"]
}
# tfclean-ignore: keep this one
import {
  id = "1234567890:default:piyo"
  to = module.foo["piyo"]
}
resource "null_resource" "bbb" {}
`),
			want: []byte(`
resource "null_resource" "aaa" {}
# tfclean-ignore: keep this one
import {
  id = "1234567890:default:piyo"
  to = module.foo["piyo"]
}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name: "tfclean-ignore only applies to the block on the immediately following line",
			data: []byte(`
resource "null_resource" "aaa" {}
# tfclean-ignore: reason

import {
  id = "resource_id"
  to = module.foo.hoge
}
resource "null_resource" "bbb" {}
`),
			want: []byte(`
resource "null_resource" "aaa" {}
# tfclean-ignore: reason

resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name: "tfclean-ignore preserves moved blocks",
			data: []byte(`
resource "null_resource" "aaa" {}
# tfclean-ignore
moved {
  from = null_resource.old
  to   = null_resource.aaa
}
resource "null_resource" "bbb" {}
`),
			want: []byte(`
resource "null_resource" "aaa" {}
# tfclean-ignore
moved {
  from = null_resource.old
  to   = null_resource.aaa
}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name: "tfclean-ignore preserves removed blocks",
			data: []byte(`
# tfclean-ignore
removed {
  from = null_resource.old

  lifecycle {
    destroy = false
  }
}
resource "null_resource" "bbb" {}
`),
			want: []byte(`
# tfclean-ignore
removed {
  from = null_resource.old

  lifecycle {
    destroy = false
  }
}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
		{
			name: "tfclean-ignore-file leaves the whole file untouched",
			data: []byte(`# tfclean-ignore-file
resource "null_resource" "aaa" {}
import {
  id = "resource_id"
  to = module.foo.hoge
}
resource "null_resource" "bbb" {}
`),
			want: []byte(`# tfclean-ignore-file
resource "null_resource" "aaa" {}
import {
  id = "resource_id"
  to = module.foo.hoge
}
resource "null_resource" "bbb" {}
`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{}
			got, err := app.applyAllDeletions(tt.data, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyAllDeletions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyAllDeletions() got = %q, want %q", got, tt.want)
			}
		})
	}
}
