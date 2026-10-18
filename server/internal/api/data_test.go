package api

import "testing"

// TestParseIntervalSeconds 桶宽解析：支持 30s/1m/5m/1h/7d，拒绝非法/超长输入
func TestParseIntervalSeconds(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"10s", 10}, {"30s", 30}, {"1m", 60}, {"5m", 300}, {"15m", 900},
		{"1h", 3600}, {"12h", 43200}, {"1d", 86400}, {"7d", 604800},
		{"", 0}, {"5", 0}, {"m", 0}, {"5x", 0}, {"abc", 0}, {"99999999d", 0},
	}
	for _, c := range cases {
		if got := parseIntervalSeconds(c.in); got != c.want {
			t.Errorf("parseIntervalSeconds(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestNiceIntervalSeconds 期望桶宽向上取整到常用整步长（10s~1d）
func TestNiceIntervalSeconds(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{1, 10}, {10, 10}, {11, 30}, {45, 60}, {61, 5 * 60},
		{400, 15 * 60}, {901, 30 * 60}, {4000, 3 * 3600}, {100000, 86400},
	}
	for _, c := range cases {
		if got := niceIntervalSeconds(c.in); got != c.want {
			t.Errorf("niceIntervalSeconds(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
