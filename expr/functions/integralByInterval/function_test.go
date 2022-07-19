package integralByInterval

import (
	"testing"

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
	tests := []th.EvalTestItem{
		{
			"integralByInterval(10s,'10s')",
			map[parser.MetricRequest][]*types.MetricData{
				{"10s", 0, 1}: {
					types.MakeMetricData("10s", []float64{1, 0, 2, 3, 4, 5, 0, 7, 8, 9, 10}, 2, 0),
				},
			},
			[]*types.MetricData{types.MakeMetricData(
				"integralByInterval(10s,'10s')",
				[]float64{1, 1, 3, 6, 10, 5, 5, 12, 20, 29, 10}, 2, 0),
			},
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
