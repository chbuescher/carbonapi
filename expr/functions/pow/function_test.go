package pow

import (
	"math"
	"testing"
	"time"

	"github.com/grafana/carbonapi/expr/helper"
	"github.com/grafana/carbonapi/expr/metadata"
	"github.com/grafana/carbonapi/expr/types"
	"github.com/grafana/carbonapi/pkg/parser"
	th "github.com/grafana/carbonapi/tests"
)

func init() {
	md := New("")
	evaluator := th.EvaluatorFromFunc(md[0].F)
	metadata.SetEvaluator(evaluator)
	helper.SetEvaluator(evaluator)
	for _, m := range md {
		metadata.RegisterFunction(m.Name, m.F)
	}
}

func TestFunction(t *testing.T) {
	now32 := int64(time.Now().Unix())

	tests := []th.EvalTestItem{
		{
			"pow(metric1,3)",
			map[parser.MetricRequest][]*types.MetricData{
				{"metric1", 0, 1}: {types.MakeMetricData("metric1", []float64{5, 1, math.NaN(), 0, 12, 125, 10.4, 1.1}, 60, now32)},
			},
			[]*types.MetricData{types.MakeMetricData("pow(metric1,3)", []float64{125, 1, math.NaN(), 0, 1728, 1953125, 1124.864, 1.331}, 60, now32)},
		},
		{
			"pow(metric1,0)",
			map[parser.MetricRequest][]*types.MetricData{
				{"metric1", 0, 1}: {types.MakeMetricData("metric1", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()}, 60, now32)},
			},
			[]*types.MetricData{types.MakeMetricData("pow(metric1,0)", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()}, 60, now32)},
		},
	}

	for _, tt := range tests {
		testName := tt.Target
		t.Run(testName, func(t *testing.T) {
			err := th.TestEvalExprModifiedOrigin(t, &tt, 0, 1, false)
			if err != nil {
				t.Errorf("unexpected error while evaluating %s: got `%+v`", tt.Target, err)
				return
			}
		})
	}

}
