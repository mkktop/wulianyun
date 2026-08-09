package rule

import (
	"encoding/json"
	"testing"
)

// TestEvalConditionTristate 三态求值：字段缺失必须返回 indeterminate（不触发也不恢复），#10 核心
func TestEvalConditionTristate(t *testing.T) {
	cond := json.RawMessage(`{"field":"temperature","op":">","value":35}`)

	cases := []struct {
		name string
		data map[string]interface{}
		want evalResult
	}{
		{"满足", map[string]interface{}{"temperature": 40.0}, evalTrue},
		{"为假", map[string]interface{}{"temperature": 20.0}, evalFalse},
		{"字段缺失(不可判定)", map[string]interface{}{"humidity": 60.0}, evalIndeterminate},
		{"值为字符串不可比较(为假)", map[string]interface{}{"temperature": "abc"}, evalFalse},
	}
	for _, tc := range cases {
		got := evalCondition(cond, tc.data)
		if got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

// TestEvalConditionCompound AND 中子条件字段缺失 → 整体不可判定
func TestEvalConditionCompoundMissing(t *testing.T) {
	cond := json.RawMessage(`{"logic":"and","conditions":[
		{"field":"temperature","op":">","value":35},
		{"field":"humidity","op":">","value":40}
	]}`)
	// 只有 temperature，缺 humidity → 不可判定（不能误判为假去恢复告警）
	if got := evalCondition(cond, map[string]interface{}{"temperature": 50.0}); got != evalIndeterminate {
		t.Errorf("AND 子条件缺失应为 indeterminate，got %d", got)
	}
	// 两者都有且都满足 → true
	if got := evalCondition(cond, map[string]interface{}{"temperature": 50.0, "humidity": 60.0}); got != evalTrue {
		t.Errorf("全满足应为 true，got %d", got)
	}
}
