// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package goldendataset

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/model/pdata"
)

func TestGenerateMetricDatas(t *testing.T) {
	mds, err := GenerateMetrics("testdata/generated_pict_pairs_metrics.txt")
	require.NoError(t, err)
	require.Equal(t, 25, len(mds))
}

func TestPICTtoCfg(t *testing.T) {
	tests := []struct {
		name   string
		inputs PICTMetricInputs
		cfg    MetricsCfg
	}{
		{
			name: "none",
			inputs: PICTMetricInputs{
				NumResourceAttrs: AttrsNone,
				NumPtsPerMetric:  NumPtsPerMetricOne,
				MetricType:       MetricTypeIntGauge,
				NumPtLabels:      LabelsNone,
			},
			cfg: MetricsCfg{
				NumResourceAttrs:     0,
				NumPtsPerMetric:      1,
				MetricDescriptorType: pdata.MetricDataTypeIntGauge,
				NumPtLabels:          0,
			},
		},
		{
			name: "one",
			inputs: PICTMetricInputs{
				NumResourceAttrs: AttrsOne,
				NumPtsPerMetric:  NumPtsPerMetricOne,
				MetricType:       MetricTypeDoubleGauge,
				NumPtLabels:      LabelsOne,
			},
			cfg: MetricsCfg{
				NumResourceAttrs:     1,
				NumPtsPerMetric:      1,
				MetricDescriptorType: pdata.MetricDataTypeGauge,
				NumPtLabels:          1,
			},
		},
		{
			name: "many",
			inputs: PICTMetricInputs{
				NumResourceAttrs: AttrsTwo,
				NumPtsPerMetric:  NumPtsPerMetricMany,
				MetricType:       MetricTypeDoubleExemplarsHistogram,
				NumPtLabels:      LabelsMany,
			},
			cfg: MetricsCfg{
				NumResourceAttrs:     2,
				NumPtsPerMetric:      16,
				MetricDescriptorType: pdata.MetricDataTypeHistogram,
				NumPtLabels:          16,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := pictToCfg(test.inputs)
			expected := test.cfg
			require.Equal(t, expected.NumResourceAttrs, actual.NumResourceAttrs)
			require.Equal(t, expected.NumPtsPerMetric, actual.NumPtsPerMetric)
			require.Equal(t, expected.MetricDescriptorType, actual.MetricDescriptorType)
			require.Equal(t, expected.NumPtLabels, actual.NumPtLabels)
		})
	}
}
