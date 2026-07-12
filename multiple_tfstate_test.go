package tfclean

import (
	"reflect"
	"strings"
	"testing"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// stateWithResource returns a minimal Terraform state JSON containing a single
// time_static resource with the given name. It is used to model "the move has
// (not) been applied in this state": when the moved-to resource is present the
// move is applied, otherwise it is still pending.
func stateWithResource(name string) string {
	return `
{
  "version": 4,
  "resources": [
    {
      "mode": "managed",
      "type": "time_static",
      "name": "` + name + `",
      "provider": "provider[\"registry.terraform.io/hashicorp/time\"]",
      "instances": [
        {
          "schema_version": 0,
          "attributes": {
            "id": "2026-05-13T13:49:53Z",
            "rfc3339": "2026-05-13T13:49:53Z"
          },
          "sensitive_attributes": [],
          "identity_schema_version": 0
        }
      ]
    }
  ]
}
`
}

// TestApp_applyAllDeletions_multipleStates covers issue #84: with several
// tfstates, a moved block is removed only when the move has been applied in all
// of them (AND semantics). A block still pending in any one state is preserved.
func TestApp_applyAllDeletions_multipleStates(t *testing.T) {
	movedBlock := []byte(`
resource "time_static" "bbb" {}

moved {
  from = time_static.aaa
  to   = time_static.bbb
}
`)
	// The move is applied where "bbb" exists, and still pending where "aaa" does.
	applied := stateWithResource("bbb")
	pending := stateWithResource("aaa")

	tests := []struct {
		name   string
		data   []byte
		states []string
		want   []byte
	}{
		{
			name:   "applied in all states: remove",
			data:   movedBlock,
			states: []string{applied, applied},
			want: []byte(`
resource "time_static" "bbb" {}

`),
		},
		{
			name:   "pending in one state: keep",
			data:   movedBlock,
			states: []string{applied, pending},
			want:   movedBlock,
		},
		{
			name:   "pending in all states: keep",
			data:   movedBlock,
			states: []string{pending, pending},
			want:   movedBlock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				hclParser: hclparse.NewParser(),
				CLI:       &CLI{},
			}
			var states []*tfstate.TFState
			for _, s := range tt.states {
				state, err := tfstate.Read(t.Context(), strings.NewReader(s))
				if err != nil {
					t.Fatal(err)
				}
				states = append(states, state)
			}
			got, err := app.applyAllDeletions(tt.data, states)
			if err != nil {
				t.Fatalf("applyAllDeletions() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyAllDeletions() got = %q, want %q", got, tt.want)
			}
		})
	}
}
