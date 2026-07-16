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

package engine

import (
	"testing"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func TestIsWithinEffectiveTime(t *testing.T) {
	mondayMorning := time.Date(2026, 7, 13, 10, 30, 0, 0, time.Local) // Monday
	sundayNight := time.Date(2026, 7, 12, 22, 0, 0, 0, time.Local)      // Sunday

	cases := []struct {
		name string
		rule model.AlertRule
		now  time.Time
		want bool
	}{
		{
			name: "empty days means all day",
			rule: model.AlertRule{EnableDaysOfWeek: "", EnableStime: "09:00", EnableEtime: "18:00"},
			now:  sundayNight,
			want: true,
		},
		{
			name: "weekday mismatch",
			rule: model.AlertRule{EnableDaysOfWeek: "1 2 3 4 5", EnableStime: "00:00", EnableEtime: "00:00"},
			now:  sundayNight,
			want: false,
		},
		{
			name: "weekday match all day",
			rule: model.AlertRule{EnableDaysOfWeek: "1 2 3 4 5", EnableStime: "00:00", EnableEtime: "00:00"},
			now:  mondayMorning,
			want: true,
		},
		{
			name: "within range",
			rule: model.AlertRule{EnableDaysOfWeek: "1", EnableStime: "09:00", EnableEtime: "18:00"},
			now:  mondayMorning,
			want: true,
		},
		{
			name: "outside range",
			rule: model.AlertRule{EnableDaysOfWeek: "1", EnableStime: "09:00", EnableEtime: "10:00"},
			now:  mondayMorning,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsWithinEffectiveTime(&tc.rule, tc.now)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
