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

package model

import (
	"reflect"
	"strconv"
	"testing"
)

func TestMakePolicy(t *testing.T) {
	type args struct {
		username string
		obj      ObjectType
		sid      string
		op       Operation
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test 1",
			args: args{
				username: "foo",
				obj:      ObjectCluster,
				sid:      "1",
				op:       OpRead,
			},
			want: []string{"foo", "clusters", "1", "read"},
		},
		{
			name: "test 2",
			args: args{
				username: "foo",
				obj:      ObjectCluster,
				sid:      "1",
				op:       OpAll,
			},
			want: []string{"foo", "clusters", "1", "*"},
		},
		{
			name: "test 3",
			args: args{
				username: "foo",
				obj:      ObjectCluster,
				sid:      "",
				op:       OpCreate,
			},
			want: []string{"foo", "clusters", "", "create"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MakePolicy(tt.args.username, tt.args.obj, tt.args.sid, tt.args.op); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakePolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetIdRangeFromPolicies(t *testing.T) {
	type args struct {
		policies [][]string
	}
	tests := []struct {
		name    string
		args    args
		wantAll bool
		wantIds []int64
	}{
		{
			name: "case 1",
			args: args{
				policies: [][]string{},
			},
			wantAll: false,
			wantIds: []int64{},
		},
		{
			name: "case 2",
			args: args{
				policies: [][]string{
					MakePolicy("foo", ObjectCluster, strconv.Itoa(1), OpRead),
				},
			},
			wantAll: false,
			wantIds: []int64{1},
		},
		{
			name: "case 3",
			args: args{
				policies: [][]string{
					MakePolicy("foo", ObjectCluster, strconv.Itoa(1), OpRead),
					MakePolicy("foo", ObjectCluster, strconv.Itoa(2), OpRead),
				},
			},
			wantAll: false,
			wantIds: []int64{1, 2},
		},
		{
			name: "case 4",
			args: args{
				policies: [][]string{
					MakePolicy("foo", ObjectCluster, "", OpRead),
				},
			},
			wantAll: false,
			wantIds: []int64{},
		},
		{
			name: "case 5",
			args: args{
				policies: [][]string{
					MakePolicy("foo", ObjectCluster, "*", OpRead),
				},
			},
			wantAll: true,
			wantIds: []int64{},
		},
		{
			name: "case 6",
			args: args{
				policies: [][]string{
					MakePolicy("foo", ObjectCluster, strconv.Itoa(1), OpRead),
					MakePolicy("foo", ObjectCluster, "*", OpRead),
				},
			},
			wantAll: true,
			wantIds: []int64{},
		},
		{
			name: "case 7",
			args: args{
				policies: [][]string{
					MakePolicy("foo", ObjectCluster, strconv.Itoa(1), OpRead),
					MakePolicy("foo", ObjectCluster, strconv.Itoa(1), OpRead),
				},
			},
			wantAll: false,
			wantIds: []int64{1},
		},
		{
			name: "case 8",
			args: args{
				policies: [][]string{
					MakePolicy("foo", ObjectCluster, strconv.Itoa(1), OpRead),
					MakePolicy("foo", ObjectCluster, strconv.Itoa(1), OpRead),
					MakePolicy("foo", ObjectCluster, "*", OpRead),
				},
			},
			wantAll: true,
			wantIds: []int64{},
		},
		{
			name: "case 9",
			args: args{
				policies: [][]string{
					MakePolicy("foo", ObjectCluster, strconv.Itoa(1), OpDelete),
					MakePolicy("foo", ObjectCluster, strconv.Itoa(2), OpUpdate),
					MakePolicy("foo", ObjectCluster, strconv.Itoa(3), OpRead),
				},
			},
			wantAll: false,
			wantIds: []int64{3},
		},
		{
			name: "case 10",
			args: args{
				policies: [][]string{
					MakePolicy("foo", ObjectCluster, strconv.Itoa(1), OpDelete),
					MakePolicy("foo", ObjectCluster, strconv.Itoa(2), OpUpdate),
					MakePolicy("foo", ObjectCluster, strconv.Itoa(3), OpRead),
					MakePolicy("foo", ObjectCluster, "*", OpRead),
				},
			},
			wantAll: true,
			wantIds: []int64{},
		},
		{
			name: "case 11",
			args: args{
				policies: [][]string{
					MakePolicy("foo", ObjectCluster, strconv.Itoa(1), OpDelete),
					MakePolicy("foo", ObjectCluster, strconv.Itoa(2), OpUpdate),
					MakePolicy("foo", ObjectCluster, strconv.Itoa(3), OpRead),
				},
			},
			wantAll: false,
			wantIds: []int64{3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAll, gotIds := GetIdRangeFromPolicies(tt.args.policies)
			if gotAll != tt.wantAll {
				t.Errorf("GetIdRangeFromPolicies() gotAll = %v, want %v", gotAll, tt.wantAll)
			}
			if !reflect.DeepEqual(gotIds, tt.wantIds) {
				t.Errorf("GetIdRangeFromPolicies() gotIds = %v, want %v", gotIds, tt.wantIds)
			}
		})
	}
}

func TestIsAdminPolicy(t *testing.T) {
	type args struct {
		policy []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "case 1",
			args: args{
				policy: MakePolicy("foo", ObjectCluster, "*", OpRead),
			},
			want: false,
		},
		{
			name: "case 1",
			args: args{
				policy: AdminPolicy,
			},
			want: true,
		},
		{
			name: "case 2",
			args: args{
				policy: []string{},
			},
			want: false,
		},
		{
			name: "case 3",
			args: args{
				policy: []string{"*", "*", "*", "*"},
			},
			want: false,
		},
		{
			name: "case 4",
			args: args{
				policy: []string{"*", "root", "*", "*"},
			},
			want: false,
		},
		{
			name: "case 5",
			args: args{
				policy: []string{"*", "*", "root", "*"},
			},
			want: false,
		},
		{
			name: "case 6",
			args: args{
				policy: []string{"*", "*", "*", "root"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAdminPolicy(tt.args.policy); got != tt.want {
				t.Errorf("IsAdminPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasAdminGroupPolicy(t *testing.T) {
	type args struct {
		policies [][]string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "case 1",
			args: args{},
			want: false,
		},
		{
			name: "case 2",
			args: args{
				policies: [][]string{
					{"foo", "bar"},
				},
			},
			want: false,
		},
		{
			name: "case 3",
			args: args{
				policies: [][]string{
					{"foo", AdminGroup},
				},
			},
			want: true,
		},
		{
			name: "case 4",
			args: args{
				policies: [][]string{
					{"foo", "bar"},
					{"foo", AdminGroup},
				},
			},
			want: true,
		},
		{
			name: "case 5",
			args: args{
				policies: [][]string{
					{"foo", "bar"},
					{"foo", "baz"},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasAdminGroupPolicy(tt.args.policies); got != tt.want {
				t.Errorf("HasAdminGroupPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}
