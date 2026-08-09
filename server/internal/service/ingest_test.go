package service

import "testing"

// TestParseClientID ClientID 为产品ID+设备名纯拼接（无分隔符）：按字符数解析，设备名可含点号
func TestParseClientID(t *testing.T) {
	cases := []struct {
		name    string
		client  string
		pk, dn  string
		ok      bool
	}{
		{"纯拼接", "AB1234567890dev1", "AB1234567890", "dev1", true},
		{"设备名含点号", "AB1234567890wifi.bathroom", "AB1234567890", "wifi.bathroom", true},
		{"多段点号", "AB1234567890a.b.c", "AB1234567890", "a.b.c", true},
		{"点号即设备名首字符(不属于平台设备)", "GY7638063548.mkk_S3", "GY7638063548", ".mkk_S3", true},
		{"产品ID非法(小写)", "ab1234567890dev1", "", "", false},
		{"产品ID非法(长度)", "ABC123456789dev1", "", "", false},
		{"无设备名", "AB1234567890", "", "", false},
		{"过短", "AB1234567890", "", "", false},
	}
	for _, tc := range cases {
		pk, dn, ok := ParseClientID(tc.client)
		if pk != tc.pk || dn != tc.dn || ok != tc.ok {
			t.Errorf("%s: ParseClientID(%q) = (%q,%q,%v), want (%q,%q,%v)",
				tc.name, tc.client, pk, dn, ok, tc.pk, tc.dn, tc.ok)
		}
	}
}
