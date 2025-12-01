package prompush

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/pkg/errors"
	"github.com/risingwavelabs/promdump/utils"
)

type Pusher interface {
	Push(ctx context.Context, reader io.Reader, pw *PushWorker, showProgress func() error, ignoreInvalidFiles bool) error
}

type NDJSONPusher struct {
}

type LegacyFormat struct {
	Metric map[string]string `json:"metric"`
	Values [][]any           `json:"values"`
}

type Item struct {
	Metric     map[string]string `json:"metric"`
	Values     []float64         `json:"values"`
	Timestamps []int64           `json:"timestamps"`
}

func (n *NDJSONPusher) Push(ctx context.Context, reader io.Reader, pw *PushWorker, showProgress func() error, ignoreInvalidFiles bool) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1*1024*1024*1024) // 1GB max line size
	for scanner.Scan() {
		line := scanner.Bytes()

		if err := showProgress(); err != nil {
			return errors.Wrap(err, "failed to show progress")
		}

		if len(line) == 0 {
			continue
		}

		var legacy LegacyFormat
		if err := json.Unmarshal(line, &legacy); err != nil {
			return fmt.Errorf("failed to unmarshal line: %w, line=%s", err, string(line))
		}
		line, err := parseLegacyFormat(&legacy)
		if err != nil {
			if errors.Is(err, ErrNaNValues) || errors.Is(err, ErrZeroTimestamp) || errors.Is(err, ErrInfValues) {
				fmt.Printf("\n%s, skipping line: %v\n", err.Error(), utils.TruncateString(string(line), 100))
				continue
			}
			return errors.Wrapf(err, "failed to parse legacy format")
		}
		pw.Push(line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	return nil
}

type AWSManagedPrometheusPusher struct {
}

type AWSManagedPrometheusQueryData struct {
	ResultType string         `json:"resultType"`
	Result     []LegacyFormat `json:"result"`
}

type AWSManagedPrometheusQueryResponse struct {
	Status string                        `json:"status"`
	Data   AWSManagedPrometheusQueryData `json:"data"`
}

func (a *AWSManagedPrometheusPusher) Push(ctx context.Context, reader io.Reader, pw *PushWorker, showProgress func() error, ignoreInvalidFiles bool) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return errors.Wrapf(err, "failed to read data")
	}
	var res AWSManagedPrometheusQueryResponse
	if err := json.Unmarshal(data, &res); err != nil {
		if ignoreInvalidFiles {
			fmt.Printf("\nfailed to unmarshal data, ignoring invalid file: %v\n", err)
			return nil
		}
		return errors.Wrapf(err, "failed to unmarshal data")
	}

	if res.Status != "success" {
		return errors.Errorf("unexpected query response status in the file: %s", res.Status)
	}

	if res.Data.ResultType != "matrix" {
		return errors.Errorf("expect `matrix` result type, got: `%s`", res.Data.ResultType)
	}

	for _, legacy := range res.Data.Result {
		line, err := parseLegacyFormat(&legacy)
		if err != nil {
			if errors.Is(err, ErrNaNValues) || errors.Is(err, ErrZeroTimestamp) || errors.Is(err, ErrInfValues) {
				fmt.Printf("\n%s, skipping line: %v\n", err.Error(), utils.TruncateString(string(line), 100))
				continue
			}
			return errors.Wrapf(err, "failed to parse legacy format")
		}
		fmt.Println(string(line))
		pw.Push(line)
	}

	return nil
}

var (
	ErrInfValues     = errors.Errorf("Inf values found")
	ErrNaNValues     = errors.Errorf("NaN values found")
	ErrZeroTimestamp = errors.Errorf("zero timestamp found")
)

func parseLegacyFormat(legacy *LegacyFormat) ([]byte, error) {
	item := Item{
		Metric: legacy.Metric,
	}

	for _, v := range legacy.Values {
		val, err := strconv.ParseFloat(v[1].(string), 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse value")
		}
		if math.IsInf(val, 0) {
			return nil, ErrInfValues
		}
		if math.IsNaN(val) {
			return nil, ErrNaNValues
		}
		item.Timestamps = append(item.Timestamps, int64(1000*v[0].(float64)))
		item.Values = append(item.Values, val)
	}
	if len(item.Timestamps) == 0 {
		return nil, ErrZeroTimestamp
	}
	l, err := json.Marshal(item)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal item")
	}
	l = append(l, '\n')
	return l, nil
}
