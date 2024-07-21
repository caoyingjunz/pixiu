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

package httputils

import "testing"

func Test_getObjectFromRequest(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantObj string
		wantSid string
		wantOk  bool
	}{
		{
			name:    "test0",
			path:    "",
			wantObj: "",
			wantOk:  false,
		},
		{
			name:    "test1",
			path:    "/",
			wantObj: "",
			wantOk:  false,
		},
		{
			name:    "test2",
			path:    "//",
			wantObj: "",
			wantOk:  false,
		},
		{
			name:    "test3",
			path:    "pixiu",
			wantObj: "",
			wantOk:  false,
		},
		{
			name:    "test4",
			path:    "/pixiu",
			wantObj: "",
			wantOk:  false,
		},
		{
			name:    "test5",
			path:    "/pixiu/",
			wantObj: "",
			wantOk:  false,
		},
		{
			name:    "test6",
			path:    "/pixiu/users",
			wantObj: "users",
			wantSid: "",
			wantOk:  true,
		},
		{
			name:    "test7",
			path:    "/pixiu/users/",
			wantObj: "users",
			wantOk:  false,
		},
		{
			name:    "test8",
			path:    "/pixiu/users/1",
			wantObj: "users",
			wantSid: "1",
			wantOk:  true,
		},
		{
			name:    "test9",
			path:    "/pixiu//",
			wantObj: "",
			wantOk:  false,
		},
		{
			name:    "test10",
			path:    "///",
			wantObj: "",
			wantOk:  false,
		},
		{
			name:    "test11",
			path:    "/pixiu/users/1/password",
			wantObj: "users",
			wantSid: "1",
			wantOk:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotObj, gotSid, gotOk := getObjectFromRequest(tt.path)
			if gotObj != tt.wantObj {
				t.Errorf("getObjectFromRequest() gotObj = %v, want %v", gotObj, tt.wantObj)
			}
			if gotSid != tt.wantSid {
				t.Errorf("getObjectFromRequest() gotSid = %v, want %v", gotSid, tt.wantSid)
			}
			if gotOk != tt.wantOk {
				t.Errorf("getObjectFromRequest() gotOk = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}
