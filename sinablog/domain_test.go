// Copyright 2026 Duc-Tam Nguyen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sinablog

import (
	"testing"
)

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "sinablog" {
		t.Errorf("Scheme = %q, want sinablog", info.Scheme)
	}
	if info.Identity.Binary != "sinablog" {
		t.Errorf("Identity.Binary = %q, want sinablog", info.Identity.Binary)
	}
	if info.Identity.Site != Host {
		t.Errorf("Identity.Site = %q, want %q", info.Identity.Site, Host)
	}
}

func TestClassify_PostURL(t *testing.T) {
	typ, id, err := Domain{}.Classify("http://blog.sina.com.cn/s/blog_abc123def.html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != "post" {
		t.Errorf("type = %q, want post", typ)
	}
	if id != "abc123def" {
		t.Errorf("id = %q, want abc123def", id)
	}
}

func TestClassify_UserURL(t *testing.T) {
	typ, id, err := Domain{}.Classify("http://blog.sina.com.cn/u/1234567")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != "user" {
		t.Errorf("type = %q, want user", typ)
	}
	if id != "1234567" {
		t.Errorf("id = %q, want 1234567", id)
	}
}

func TestClassify_Empty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestLocate_Post(t *testing.T) {
	got, err := Domain{}.Locate("post", "abc123def")
	want := "http://blog.sina.com.cn/s/blog_abc123def.html"
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLocate_User(t *testing.T) {
	got, err := Domain{}.Locate("user", "1234567")
	want := "http://blog.sina.com.cn/u/1234567"
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLocate_UnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "123")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}
