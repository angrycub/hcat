package tfunc

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/hashicorp/hcat"
)

func TestTransformExecute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ti   hcat.TemplateInput
		i    hcat.Recaller
		e    string
		err  bool
	}{
		{
			"func_base64Decode",
			hcat.TemplateInput{
				Contents: `{{ base64Decode "aGVsbG8=" }}`,
			},
			nil,
			"hello",
			false,
		},
		{
			"func_base64Decode_bad",
			hcat.TemplateInput{
				Contents: `{{ base64Decode "aGVsxxbG8=" }}`,
			},
			nil,
			"",
			true,
		},
		{
			"func_base64Encode",
			hcat.TemplateInput{
				Contents: `{{ base64Encode "hello" }}`,
			},
			nil,
			"aGVsbG8=",
			false,
		},
		{
			"func_base64URLDecode",
			hcat.TemplateInput{
				Contents: `{{ base64URLDecode "dGVzdGluZzEyMw==" }}`,
			},
			nil,
			"testing123",
			false,
		},
		{
			"func_base64URLDecode_bad",
			hcat.TemplateInput{
				Contents: `{{ base64URLDecode "aGVsxxbG8=" }}`,
			},
			nil,
			"",
			true,
		},
		{
			"func_base64URLEncode",
			hcat.TemplateInput{
				Contents: `{{ base64URLEncode "testing123" }}`,
			},
			nil,
			"dGVzdGluZzEyMw==",
			false,
		},
		{
			"helper_toJSON",
			hcat.TemplateInput{
				Contents: `{{ "a,b,c" | split "," | toJSON }}`,
			},
			hcat.NewStore(),
			"[\"a\",\"b\",\"c\"]",
			false,
		},
		{
			"helper_toLower",
			hcat.TemplateInput{
				Contents: `{{ "HI" | toLower }}`,
			},
			hcat.NewStore(),
			"hi",
			false,
		},
		{
			"helper_toTitle",
			hcat.TemplateInput{
				Contents: `{{ "this is a sentence" | toTitle }}`,
			},
			hcat.NewStore(),
			"This Is A Sentence",
			false,
		},
		{
			"helper_toTOML",
			hcat.TemplateInput{
				Contents: `{{ "{\"foo\":\"bar\"}" | parseJSON | toTOML }}`,
			},
			hcat.NewStore(),
			"foo = \"bar\"",
			false,
		},
		{
			"helper_toUpper",
			hcat.TemplateInput{
				Contents: `{{ "hi" | toUpper }}`,
			},
			hcat.NewStore(),
			"HI",
			false,
		},
		{
			"helper_toYAML",
			hcat.TemplateInput{
				Contents: `{{ "{\"foo\":\"bar\"}" | parseJSON | toYAML }}`,
			},
			hcat.NewStore(),
			"foo: bar",
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			tpl := hcat.NewTemplate(tc.ti)

			a, err := tpl.Execute(tc.i)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}
			if a != nil && !bytes.Equal([]byte(tc.e), a.Output) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.e, string(a.Output))
			}
		})
	}
}