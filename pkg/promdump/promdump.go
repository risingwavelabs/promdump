package promdump

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom_model "github.com/prometheus/common/model"
)

const PrometheusDefaultMaxResolution = 11_000

type DumpOpt struct {
	Endpoint     string
	Start        time.Time
	End          time.Time
	Step         time.Duration
	Query        string
	MetricsNames []string
	Gzip         bool
	MemoryRatio  float32
}

func DumpToFileWithCallback(ctx context.Context, opt *DumpOpt, filename string, cb QueryCallback) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to open file")
	}
	defer f.Close()

	return DumpToWriter(ctx, opt, f, cb)
}

func DumpToWriter(ctx context.Context, opt *DumpOpt, writer io.Writer, cb QueryCallback) error {
	var w io.Writer
	if opt.Gzip {
		gw := gzip.NewWriter(writer)
		defer gw.Close()
		w = gw
	} else {
		w = writer
	}

	isFirstItem := true
	if err := dump(ctx, opt, func(query string, value prom_model.Matrix, progress float32) error {
		write := func(p []byte) error {
			_, err := w.Write(p)
			return err
		}

		if !isFirstItem {
			if err := write([]byte("\n")); err != nil {
				return errors.Wrapf(err, "failed to write comma")
			}
		} else {
			isFirstItem = false
		}

		for i, series := range value {
			raw, err := json.Marshal(series)
			if err != nil {
				return errors.Wrapf(err, "failed to marshal value")
			}
			if len(raw) == 0 {
				continue
			}
			if err := write(raw); err != nil {
				return errors.Wrapf(err, "failed to write value")
			}
			if i < len(value)-1 {
				if err := write([]byte("\n")); err != nil {
					return errors.Wrapf(err, "failed to write newline")
				}
			}
		}

		if cb != nil {
			if err := cb(query, value, progress); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return errors.Wrapf(err, "failed to dump")
	}
	return nil
}

type QueryCallback func(query string, value prom_model.Matrix, progress float32) error

func dump(ctx context.Context, opt *DumpOpt, cb QueryCallback) error {
	client, err := api.NewClient(api.Config{
		Address: opt.Endpoint,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to create prometheus client")
	}

	v1api := v1.NewAPI(client)

	// construct queries
	var queries []string
	if len(opt.Query) > 0 {
		queries = []string{opt.Query}
	} else if len(opt.MetricsNames) > 0 {
		fmt.Println("metrics names", opt.MetricsNames)
		for _, metric := range opt.MetricsNames {
			metricName := strings.TrimSpace(metric)
			if metricName == "" {
				continue
			}
			queries = append(queries, metricName)
		}
	} else { // get all metric names
		labelValues, warnings, err := v1api.LabelValues(ctx, "__name__", []string{}, opt.Start, opt.End)
		if err != nil {
			return errors.Wrapf(err, "failed to get label values")
		}
		if len(warnings) > 0 {
			return errors.Errorf("warnings: %v", warnings)
		}
		for _, labelValue := range labelValues {
			queries = append(queries, string(labelValue))
		}
	}

	// calculate query chunks
	timeRanges := calTimeRanges(opt.Start, opt.End, opt.Step, opt.MemoryRatio)

	// run all queries
	for qi, query := range queries {
		vs, warnings, err := queryAndMerge(ctx, v1api, string(query), opt.Step, timeRanges)
		if err != nil {
			return errors.Wrapf(err, "failed to query range")
		}
		if len(warnings) > 0 {
			return errors.Errorf("warnings: %v", warnings)
		}
		// traverse all matrices
		for vi, v := range vs {
			matrix, ok := v.(prom_model.Matrix)
			if !ok {
				return errors.New("value is not a matrix")
			}
			progress := float32(qi+1)/float32(len(queries)) +
				float32(vi+1)/float32(len(vs))*(1/float32(len(queries)))
			if cb != nil {
				if err := cb(string(query), matrix, progress); err != nil {
					return errors.Wrapf(err, "failed to run callback")
				}
			}
		}
	}
	return nil
}

// queryAndMerge queries all time ranges and then merge the results
func queryAndMerge(ctx context.Context, v1api v1.API, query string, step time.Duration, timeRanges []TimeRange, opts ...v1.Option) ([]prom_model.Value, v1.Warnings, error) {
	var vs []prom_model.Value
	var retWarnings v1.Warnings
	for _, timeRange := range timeRanges {
		v, warnings, err := v1api.QueryRange(ctx, query, v1.Range{
			Start: timeRange.Start,
			End:   timeRange.End,
			Step:  step,
		}, opts...)
		if err != nil {
			return nil, warnings, errors.Wrapf(err, "failed to query range")
		}
		vs = append(vs, v)
		retWarnings = append(retWarnings, warnings...)
	}
	return vs, retWarnings, nil
}
