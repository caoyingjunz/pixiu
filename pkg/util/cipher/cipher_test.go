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

/*
 @Version : 1.0
 @Author  : steven.wang
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2022/2022/14 14/32/15
 @Desc    :
*/

package cipher

import "testing"

var (
	kb1 = `aaaaaaa`
	kb2 = `bbbbbbb`
	kb3 = `ccccccc`
)

func TestEncrypt(t *testing.T) {
	cases := []struct {
		Name string
		text []byte
	}{
		{"a", []byte(kb1)},
		{"b", []byte(kb2)},
		{"c", []byte(kb3)},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if ans, err := Encrypt(c.text); err != nil {
				t.Fatalf("encrypt text %s failed: %+v",
					c.text, err)
			} else {
				t.Logf("encrypt text %s is { %s }", c.text, ans)
			}
		})
	}
}

func TestDecrypt(t *testing.T) {
	cases := []struct {
		Name string
		text string
	}{
		{"a", "Obx1VwUPs7B09CqalouHQg=="},
		{"b", "Zol2IPDQuGTo/K0IYDkkAQ=="},
		{"c", "nmW+Ha3epblxZmgVvcvaSQ=="},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if ans, err := Decrypt(c.text); err != nil {
				t.Fatalf("decrypt text %s failed: %+v",
					c.text, err)
			} else {
				t.Logf("decrypt text %s is %s", c.text, ans)
			}
		})
	}
}
