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

package pixiu

import (
	"testing"
	"time"
)

func TestLocalTimeScanString(t *testing.T) {
	var lt LocalTime
	if err := lt.Scan("2026-08-04 10:07:04.190348"); err != nil {
		t.Fatalf("Scan string: %v", err)
	}
	if lt.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if got := lt.Format("2006-01-02 15:04:05"); got != "2026-08-04 10:07:04" {
		t.Fatalf("got %s", got)
	}
}

func TestLocalTimeScanTime(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.Local)
	var lt LocalTime
	if err := lt.Scan(now); err != nil {
		t.Fatalf("Scan time.Time: %v", err)
	}
	if !lt.StdTime().Equal(now) {
		t.Fatalf("mismatch: %v vs %v", lt.StdTime(), now)
	}
}
