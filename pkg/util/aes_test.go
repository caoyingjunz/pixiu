/*
 @Version : 1.0
 @Author  : steven.wang
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2022/2022/14 14/32/15
 @Desc    :
*/

package util

import "testing"

func TestAesCBCEncrypt(t *testing.T) {
	cases := []struct {
		Name string
		text string
	}{
		{"a", "aaaaaa"},
		{"b", "bbbbbb"},
		{"c", "cccccc"},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if ans, err := AesCBCEncrypt(c.text); err != nil {
				t.Fatalf("encrypt text %s failed: %+v",
					c.text, err)
			} else {
				t.Logf("encrypt text %s is %s", c.text, ans)
			}
		})
	}
}
