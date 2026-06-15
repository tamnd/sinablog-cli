package sinablog

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "sinablog" {
		t.Errorf("Scheme = %q, want sinablog", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "sinablog" {
		t.Errorf("Identity.Binary = %q, want sinablog", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"https://blog.sina.com.cn/s/blog_abc123def.html", "article", "blog_abc123def"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("article", "blog_abc123def")
	want := "https://blog.sina.com.cn/s/blog_abc123def.html"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	info, ok := h.Domain("sinablog")
	if !ok {
		t.Fatal("sinablog domain not registered")
	}
	if info.Identity.Binary != "sinablog" {
		t.Errorf("Binary = %q, want sinablog", info.Identity.Binary)
	}
}
