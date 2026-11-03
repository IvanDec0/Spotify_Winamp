package skins

import (
	"strings"
	"testing"
)

func TestValidateCSSAcceptsLocalAssets(t *testing.T) {
	source := `:host {
  --accent: #ffb347;
  --background: url("assets/bg.png");
  background: url(assets/bg.png);
}
@media (min-width: 380px) {
  .cad-titlebar { color: color-mix(in srgb, var(--accent) 70%, white); }
}
@font-face { font-family: Afterglow; src: local("Inter"), url(assets/font.woff2) format("woff2"); }
@keyframes pulse { from { opacity: 0.2; } to { opacity: 1; } }
`
	assets, err := ValidateCSS(source)
	if err != nil {
		t.Fatalf("ValidateCSS() error = %v", err)
	}
	if len(assets) != 2 || assets[0] != "assets/bg.png" || assets[1] != "assets/font.woff2" {
		t.Fatalf("assets = %#v", assets)
	}
}

func TestValidateCSSRejectsUnsafeConstructs(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "import", source: `@import url("other.css");`, want: "@import"},
		{name: "escaped import", source: `@\69 mport url("other.css");`, want: "@import"},
		{name: "remote url", source: `.a { background: url(https://evil.test/x.png); }`, want: "remote or absolute"},
		{name: "protocol relative", source: `.a { background: url(//evil.test/x.png); }`, want: "remote or absolute"},
		{name: "absolute", source: `.a { background: url(/etc/passwd); }`, want: "remote or absolute"},
		{name: "data url", source: `.a { background: url(data:image/png;base64,AAAA); }`, want: "remote or absolute"},
		{name: "javascript", source: `.a { background: url(javascript:alert(1)); }`, want: "script URL"},
		{name: "unsupported asset", source: `.a { background: url(assets/icon.svg); }`, want: "unsupported asset type"},
		{name: "custom property remote", source: `:host { --background: url(https://evil.test/x.png); }`, want: "remote or absolute"},
		{name: "expression", source: `.a { width: expression(alert(1)); }`, want: "expression"},
		{name: "behavior", source: `.a { behavior: url(assets/bg.png); }`, want: "behavior"},
		{name: "moz binding", source: `.a { -moz-binding: url(assets/bg.png); }`, want: "binding"},
		{name: "forbidden at rule", source: `@namespace svg url(http://www.w3.org/2000/svg);`, want: "at-rule"},
		{name: "bad function", source: `.a { background: image-set("https://evil.test/x.png" 1x); }`, want: "image-set"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ValidateCSS(test.source)
			if err == nil {
				t.Fatal("ValidateCSS() error = nil")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want containing %q", err, test.want)
			}
		})
	}
}

func TestValidateCSSRejectsEscapedJavascript(t *testing.T) {
	_, err := ValidateCSS(`.a { background: url(\6a avascript:alert(1)); }`)
	if err == nil {
		t.Fatal("ValidateCSS() error = nil")
	}
}

func TestDecodeCSSEscapes(t *testing.T) {
	if got := decodeCSSEscapes(`\6a avascript\3a`); got != "javascript:" {
		t.Fatalf("decodeCSSEscapes() = %q", got)
	}
}
