package promdump

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
)

type DumpMultipartCfg struct {
	Opt       *DumpOpt
	Parts     int
	OutputDir string
	Verbose   bool
}

func validateDumpOptions(cfg *DumpMultipartCfg) error {
	opt := cfg.Opt
	if cfg.Parts <= 0 {
		return errors.New("parts must be greater than 0")
	}
	if opt.Start.After(opt.End) {
		return errors.New("start time must be before end time")
	}
	if opt.Step <= 0 {
		return errors.New("step must be greater than 0")
	}
	if opt.Endpoint == "" {
		return errors.New("endpoint must be provided")
	}
	return nil
}

func getOutputDir(cfg *DumpMultipartCfg) (string, error) {
	opt := cfg.Opt
	if cfg.OutputDir == "" {
		return "", fmt.Errorf("out is required")
	}
	if cfg.OutputDir == "." {
		wd, err := os.Getwd()
		if err != nil {
			return "", errors.Wrap(err, "failed to get current directory")
		}
		// digest
		digest := sha256.New()
		digest.Write([]byte(fmt.Sprintf("%+v", opt)))
		digestStr := hex.EncodeToString(digest.Sum(nil))[:8]
		return filepath.Join(wd, fmt.Sprintf("promdump_%s", digestStr)), nil
	} else { // user specified output directory
		if !filepath.IsAbs(cfg.OutputDir) {
			outDir, err := filepath.Abs(cfg.OutputDir)
			if err != nil {
				return "", errors.Wrap(err, "failed to get absolute path")
			}
			return outDir, nil
		}
		return cfg.OutputDir, nil
	}
}

type DumpProgressCallback func(curr int, total int, progress float32) error

func DumpMultipart(ctx context.Context, cfg *DumpMultipartCfg, cb DumpProgressCallback) error {
	opt := cfg.Opt

	v := func(format string, args ...any) {
		if cfg.Verbose {
			fmt.Printf(format, args...)
		}
	}

	v("Dumping Prometheus data from %s to %s\n", opt.Endpoint, cfg.OutputDir)
	v("Time range: %s to %s with step %s\n", opt.Start.Format(time.RFC3339), opt.End.Format(time.RFC3339), opt.Step)

	if err := validateDumpOptions(cfg); err != nil {
		return errors.Wrap(err, "invalid dump options")
	}

	outDir, err := getOutputDir(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to get output directory")
	}

	if cfg.Parts == 1 { // output to a file
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return errors.Wrap(err, "failed to create output directory")
		}
		outFile := filepath.Join(outDir, "promdump.ndjson")
		if opt.Gzip {
			outFile += ".gz"
		}

		// Execute the dump
		err = DumpToFileWithCallback(ctx, opt, outFile, func(query string, value model.Matrix, progress float32) error {
			if cb != nil {
				return cb(1, 1, progress)
			}
			return nil
		})
		if err != nil {
			return errors.Wrap(err, "failed to dump prometheus data")
		}
		v("Successfully dumped prometheus data to %s\n", outFile)
	} else { // output to a folder
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return errors.Wrap(err, "failed to create output directory")
		}
		files, err := os.ReadDir(outDir)
		if err != nil {
			return errors.Wrap(err, "failed to read output directory")
		}
		maxPart := 0
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			// trim the .ndjson.gz suffix
			part, err := strconv.Atoi(
				strings.TrimSuffix(
					strings.TrimSuffix(
						file.Name(),
						".gz",
					),
					".ndjson",
				),
			)
			if err != nil {
				continue
			}
			if part > maxPart {
				maxPart = part
			}
		}

		// split the start and end time into parts
		var timeRanges [][]time.Time
		rangeInterval := opt.End.Sub(opt.Start) / time.Duration(cfg.Parts)
		for i := 0; i < cfg.Parts; i++ {
			timeRanges = append(timeRanges, []time.Time{
				opt.Start.Add(time.Duration(i) * rangeInterval),
				opt.Start.Add(time.Duration(i+1) * rangeInterval),
			})
		}

		for i, timeRange := range timeRanges {
			v("Dumping part %d (%d/%d) %s to %s\n", i, i+1, cfg.Parts, timeRange[0].Format(time.RFC3339), timeRange[1].Format(time.RFC3339))
			if i < maxPart {
				continue
			}
			outFile := filepath.Join(outDir, fmt.Sprintf("%d.ndjson", i))
			if opt.Gzip {
				outFile += ".gz"
			}

			// Execute the dump
			err = DumpToFileWithCallback(ctx, opt, outFile, func(query string, value model.Matrix, progress float32) error {
				if cb != nil {
					return cb(i+1, cfg.Parts, progress)
				}
				return nil
			})
			if err != nil {
				return errors.Wrap(err, "failed to dump prometheus data")
			}
		}
	}

	return nil
}
