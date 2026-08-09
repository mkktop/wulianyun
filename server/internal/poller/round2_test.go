package poller

import (
	"math"
	"testing"
)

// TestRound2NegativeBias round2 对负数必须对称舍入（不向零偏置），#20
func TestRound2NegativeBias(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0.005, 0.01},
		{-0.005, -0.01}, // 原实现 int64(v*100+0.5) 会得 0（向零偏置），掩盖冷链冻融阈值
		{-0.515, -0.52},
		{-0.525, -0.53}, // math.Round 半数远离零：-52.5 → -53
		{1.235, 1.24},
		{23.626, 23.63},
	}
	for _, tc := range cases {
		got := round2(tc.in)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("round2(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
