/*
Copyright 2024 The Pixiu Authors.

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

package db

import (
	"fmt"
	"testing"
)

func TestIsKingbaseUdtNameError(t *testing.T) {
	err := fmt.Errorf("ERROR: column c.udt_name does not exist (SQLSTATE 42703)")
	if !isKingbaseUdtNameError(err) {
		t.Fatal("expected kingbase udt_name error to match")
	}
	if isKingbaseUdtNameError(fmt.Errorf("other error")) {
		t.Fatal("unexpected match")
	}
	if isKingbaseUdtNameError(nil) {
		t.Fatal("nil should not match")
	}
}

func TestNormalizeSQLDataType(t *testing.T) {
	cases := map[string]string{
		"character varying":           "varchar",
		"INTEGER":                     "int4",
		"timestamp without time zone": "timestamp",
		"double precision":            "float8",
		"text":                        "text",
	}
	for in, want := range cases {
		if got := normalizeSQLDataType(in); got != want {
			t.Fatalf("normalizeSQLDataType(%q)=%q, want %q", in, got, want)
		}
	}
}
