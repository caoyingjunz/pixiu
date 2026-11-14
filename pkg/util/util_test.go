/*
Copyright 2021 The Pixiu Authors.

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

package util

import (
	"regexp"
	"testing"
)

func TestGenerateRequestID(t *testing.T) {
	pattern := regexp.MustCompile(`^\d{14}-\d{6}$`)
	for i := 0; i < 10; i++ {
		id := GenerateRequestID()
		if !pattern.MatchString(id) {
			t.Fatalf("unexpected request id format: %q", id)
		}
	}
}

func TestIsEmptyS(t *testing.T) {
	if IsEmptyS("") {
		t.Fatal("empty string should be reported as empty")
	}
	if !IsEmptyS("pixiu") {
		t.Fatal("non-empty string should not be reported as empty")
	}
}

func TestMoreAndLess(t *testing.T) {
	if got := More(3, 7); got != 7 {
		t.Fatalf("More(3, 7) = %d, want 7", got)
	}
	if got := Less(3, 7); got != 3 {
		t.Fatalf("Less(3, 7) = %d, want 3", got)
	}
	if More(5, 5) != 5 || Less(5, 5) != 5 {
		t.Fatal("More/Less should return the same value for equal inputs")
	}
}

func TestDeduplicateIntSlice(t *testing.T) {
	got := DeduplicateIntSlice([]int64{3, 1, 3, 2, 1, 5})
	want := []int64{3, 1, 2, 5}
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected result: got %v, want %v", got, want)
		}
	}
}

func TestValidateStrongPassword(t *testing.T) {
	cases := []struct {
		password string
		want     bool
	}{
		{"Pixiu123", true},
		{"pixiu123", false},  // no upper case letter
		{"PIXIU123", false},  // no lower case letter
		{"Pixiuabc", false},  // no number
		{"Pix1", false},      // too short
	}
	for _, c := range cases {
		if got := ValidateStrongPassword(c.password); got != c.want {
			t.Fatalf("ValidateStrongPassword(%q) = %v, want %v", c.password, got, c.want)
		}
	}
}
