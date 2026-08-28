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

package loginlimit

import "testing"

func TestAllowUserLimitsBurst(t *testing.T) {
	name := "admin-flood-test-user"
	allowed := 0
	for i := 0; i < 20; i++ {
		if AllowUser(name) {
			allowed++
		}
	}
	if allowed != userBurst {
		t.Fatalf("expected burst=%d allows, got %d", userBurst, allowed)
	}
}

func TestAllowIPLimitsBurst(t *testing.T) {
	ip := "203.0.113.99"
	allowed := 0
	for i := 0; i < 20; i++ {
		if AllowIP(ip) {
			allowed++
		}
	}
	if allowed != ipBurst {
		t.Fatalf("expected burst=%d allows, got %d", ipBurst, allowed)
	}
}

func TestAcquireVerifyCap(t *testing.T) {
	acquired := 0
	for i := 0; i < maxConcurrentVerify; i++ {
		if !AcquireVerify() {
			t.Fatalf("expected acquire %d to succeed", i)
		}
		acquired++
	}
	if AcquireVerify() {
		t.Fatal("expected acquire beyond cap to fail")
	}
	for i := 0; i < acquired; i++ {
		ReleaseVerify()
	}
	if !AcquireVerify() {
		t.Fatal("expected acquire after release to succeed")
	}
	ReleaseVerify()
}
