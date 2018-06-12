package barrister

import (
	"testing"
	"fmt"
	"bytes"
)

func TestGenerateEnum(t *testing.T) {
	for i, tc := range []struct {
		enums map[string][]EnumValue
		res []byte
	}{
		{
			map[string][]EnumValue{
				"asdf": {EnumValue{Value: "foo"}},
			},
			[]byte("type Asdf string\nconst (\n	AsdfFoo Asdf = \"foo\"\n)\n\n"),
	},
		{
			map[string][]EnumValue{
				"asdf": {EnumValue{Value: "foo"}, EnumValue{Value: "bar"}},
			},
			[]byte("type Asdf string\nconst (\n	AsdfFoo Asdf = \"foo\"\n	AsdfBar Asdf = \"bar\"\n)\n\n"),
		},
	} {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()

			g := &generateGo{
				idl: &Idl{
					enums: tc.enums,
				},
			}

			res := &bytes.Buffer{}
			g.generateEnum(res, "asdf")

			if string(res.Bytes()) != string(tc.res) {
				t.Errorf("Expected %s, got %s", tc.res, res.Bytes())
			}
		})
	}
}
