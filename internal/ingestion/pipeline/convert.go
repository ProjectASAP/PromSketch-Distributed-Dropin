package pipeline

import (
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"

	vmprompb "github.com/zzylol/VictoriaMetrics/lib/prompb"
	"github.com/zzylol/VictoriaMetrics/lib/prompbmarshal"
)

// vmPrompbToPrompb converts VM remote write parser output to Prometheus prompb types.
// This copies all data so the result is safe to hold after the VM parser callback returns.
func vmPrompbToPrompb(tss []vmprompb.TimeSeries) []prompb.TimeSeries {
	result := make([]prompb.TimeSeries, len(tss))
	for i, ts := range tss {
		lbls := make([]prompb.Label, len(ts.Labels))
		for j, l := range ts.Labels {
			lbls[j] = prompb.Label{Name: l.Name, Value: l.Value}
		}
		samples := make([]prompb.Sample, len(ts.Samples))
		for j, s := range ts.Samples {
			samples[j] = prompb.Sample{Value: s.Value, Timestamp: s.Timestamp}
		}
		result[i] = prompb.TimeSeries{Labels: lbls, Samples: samples}
	}
	return result
}

// vmMarshalToPrompb converts VM OTLP/scrape parser output to Prometheus prompb types.
// This copies all data so the result is safe to hold after the VM parser callback returns.
func vmMarshalToPrompb(tss []prompbmarshal.TimeSeries) []prompb.TimeSeries {
	result := make([]prompb.TimeSeries, len(tss))
	for i, ts := range tss {
		lbls := make([]prompb.Label, len(ts.Labels))
		for j, l := range ts.Labels {
			lbls[j] = prompb.Label{Name: l.Name, Value: l.Value}
		}
		samples := make([]prompb.Sample, len(ts.Samples))
		for j, s := range ts.Samples {
			samples[j] = prompb.Sample{Value: s.Value, Timestamp: s.Timestamp}
		}
		result[i] = prompb.TimeSeries{Labels: lbls, Samples: samples}
	}
	return result
}

// VMMarshalLabelsToLabels converts VM prompbmarshal labels to Prometheus model labels.
// Exported for use by pskinsert.
func VMMarshalLabelsToLabels(vmLabels []prompbmarshal.Label) labels.Labels {
	lbls := make(labels.Labels, len(vmLabels))
	for i, l := range vmLabels {
		lbls[i] = labels.Label{Name: l.Name, Value: l.Value}
	}
	return lbls
}

// VMMarshalTSToPrompb converts a single VM prompbmarshal TimeSeries to a Prometheus prompb TimeSeries.
// Exported for use by pskinsert.
func VMMarshalTSToPrompb(ts *prompbmarshal.TimeSeries) *prompb.TimeSeries {
	lbls := make([]prompb.Label, len(ts.Labels))
	for i, l := range ts.Labels {
		lbls[i] = prompb.Label{Name: l.Name, Value: l.Value}
	}
	samples := make([]prompb.Sample, len(ts.Samples))
	for i, s := range ts.Samples {
		samples[i] = prompb.Sample{Value: s.Value, Timestamp: s.Timestamp}
	}
	return &prompb.TimeSeries{Labels: lbls, Samples: samples}
}
