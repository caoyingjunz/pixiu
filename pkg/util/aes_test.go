/*
 @Version : 1.0
 @Author  : steven.wang
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2022/2022/14 14/32/15
 @Desc    :
*/

package util

import "testing"

var (
	kb1 = `aaaaaaa`
	kb2 = `bbbbbbb`
	kb3 = `ccccccc`
)

func TestAesCBCEncrypt(t *testing.T) {
	cases := []struct {
		Name string
		text string
	}{
		{"a", kb1},
		{"b", kb2},
		{"c", kb3},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if ans, err := AesCBCEncrypt(c.text); err != nil {
				t.Fatalf("encrypt text %s failed: %+v",
					c.text[:10], err)
			} else {
				t.Logf("encrypt text %s is { %s }", c.text[:10], ans)
			}
		})
	}
}

func TestAesCBCDecrypt(t *testing.T) {
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
			if ans, err := AesCBCDecrypt(c.text); err != nil {
				t.Fatalf("decrypt text %s failed: %+v",
					c.text, err)
			} else {
				t.Logf("decrypt text %s is %s", c.text, ans)
			}
		})
	}
}
