package memory

import (
	"math"
	"testing"
)

func TestBM25ParametersMatchMemoryContract(t *testing.T) {
	tests := []struct {
		// name 是测试场景名称。
		name string
		// query 是等待计算参数的查询文本。
		query string
		// midpoint 是 当前查询词数对应的 sigmoid 中点。
		midpoint float64
		// steepness 是 当前查询词数对应的 sigmoid 陡峭度。
		steepness float64
	}{
		{name: "short", query: "one two three", midpoint: 5, steepness: 0.7},
		{name: "medium", query: "one two three four", midpoint: 7, steepness: 0.6},
		{name: "long", query: "one two three four five six seven", midpoint: 9, steepness: 0.5},
		{name: "very long", query: "one two three four five six seven eight nine ten", midpoint: 10, steepness: 0.5},
		{name: "largest", query: "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen", midpoint: 12, steepness: 0.5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			midpoint, steepness := bm25Parameters(test.query)
			if midpoint != test.midpoint || steepness != test.steepness {
				t.Fatalf("unexpected parameters: got (%v, %v), want (%v, %v)", midpoint, steepness, test.midpoint, test.steepness)
			}
		})
	}
}

func TestNormalizeBM25MatchesMemoryContractSigmoid(t *testing.T) {
	if score := normalizeBM25(5, 5, 0.7); math.Abs(score-0.5) > 1e-12 {
		t.Fatalf("midpoint score must be 0.5, got %v", score)
	}
	low := normalizeBM25(1, 5, 0.7)
	high := normalizeBM25(10, 5, 0.7)
	if low <= 0 || high >= 1 || low >= high {
		t.Fatalf("scores must stay in (0,1) and increase monotonically: low=%v high=%v", low, high)
	}
}
