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

package intstr

import (
	"fmt"
	"strconv"
)

type IntOrString struct {
	Type   Type
	IntVal int64
	StrVal string
}

// Type represents the stored type of IntOrString.
type Type int

const (
	Int64  Type = iota // The IntOrString holds an int64.
	String             // The IntOrString holds a string.
)

func FromInt64(val int64) IntOrString {
	return IntOrString{Type: Int64, IntVal: val}
}

// FromString creates an IntOrString object with a string value.
func FromString(val string) IntOrString {
	return IntOrString{Type: String, StrVal: val}
}

// String returns the string value, or the Itoa of the int value.
func (intstr *IntOrString) String() string {
	if intstr.Type == String {
		return intstr.StrVal
	}
	return fmt.Sprintf("%d", intstr.Int64())
}

func (intstr *IntOrString) Int64() int64 {
	if intstr.Type == String {
		i, _ := strconv.ParseInt(intstr.StrVal, 10, 64)
		return i
	}
	return intstr.IntVal
}
