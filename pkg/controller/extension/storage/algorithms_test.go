/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage

import (
	"reflect"
	"testing"
)

func TestCompactDeleteKeys(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"去重", []string{"a", "a", "b"}, []string{"a", "b"}},
		{"剔除空键与纯空白", []string{"", "   ", "a"}, []string{"a"}},
		{"裁剪首尾空白", []string{" a ", "a"}, []string{"a"}},
		{"目录覆盖其下全部对象", []string{"dir/", "dir/x", "dir/y/z"}, []string{"dir/"}},
		{"foo 与 foo/ 是不同 key 不互相覆盖", []string{"foo", "foo/"}, []string{"foo", "foo/"}},
		{"普通前缀不误判为目录覆盖", []string{"a/", "ab"}, []string{"a/", "ab"}},
		{"输入顺序不影响结果", []string{"dir/b", "dir/", "dir/a"}, []string{"dir/"}},
		{"全部为空返回空", []string{"", " "}, []string{}},
	}
	for _, tc := range cases {
		if got := compactDeleteKeys(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: compactDeleteKeys(%v) = %v, want %v", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestSanitizeArchiveName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"file.txt", "file.txt"},
		{"dir/file.txt", "dir/file.txt"},
		{"../evil", "evil"},
		{"..", "object"},
		{"", "object"},
		{"/", "object"},
		{"/abs/path", "abs/path"},
		{"./x/./y", "x/y"},
		{`a\b`, "a/b"},
		{`..\..\etc/passwd`, "etc/passwd"},
	}
	for _, tc := range cases {
		if got := sanitizeArchiveName(tc.in); got != tc.want {
			t.Errorf("sanitizeArchiveName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBaseObjectName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"file", "file"},
		{"dir/file.txt", "file.txt"},
		{"a/b/c", "c"},
		{"dir/sub/", "sub"},
		{" dir/f.txt ", "f.txt"},
	}
	for _, tc := range cases {
		if got := baseObjectName(tc.in); got != tc.want {
			t.Errorf("baseObjectName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeObjectPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"/", ""},
		{"a", "a/"},
		{"/a/b/", "a/b/"},
		{" a/b ", "a/b/"},
	}
	for _, tc := range cases {
		if got := normalizeObjectPrefix(tc.in); got != tc.want {
			t.Errorf("normalizeObjectPrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMatchVersionPrefix(t *testing.T) {
	cases := []struct {
		key    string
		prefix string
		want   bool
	}{
		{"anything", "", true},
		{"a/b.txt", "a/b.txt", true},
		{"a/b.txt2", "a/b.txt", false},
		{"a/b.txt/1", "a/b.txt", true},
		{"dir/x", "dir/", true},
		{"dir", "dir/", false},
		{"dirx/y", "dir/", false},
	}
	for _, tc := range cases {
		if got := matchVersionPrefix(tc.key, tc.prefix); got != tc.want {
			t.Errorf("matchVersionPrefix(%q, %q) = %v, want %v", tc.key, tc.prefix, got, tc.want)
		}
	}
}

func TestValidBucketName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"abc", true},
		{"a-b.c1", true},
		{"ab", false},
		{"a..b", false},
		{"192.168.0.1", false},
		{"A-bc", false},
		{"-abc", false},
		{"abc-", false},
		{"a_bc", false},
		{"abc.", false},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false}, // 64 字符
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},   // 63 字符
	}
	for _, tc := range cases {
		if got := validBucketName(tc.name); got != tc.want {
			t.Errorf("validBucketName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
