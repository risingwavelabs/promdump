package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/risingwavelabs/promdump/pkg/promdump"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:   "promdump",
		Usage:  "Dump Prometheus data to a *.ndjson.gz file, and serve the file as a Prometheus remote_read endpoint",
		Action: runDump,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "out",
				Aliases: []string{"o"},
				Usage:   "Output directory",
				Value:   ".",
			},
			&cli.StringFlag{
				Name:     "endpoint",
				Aliases:  []string{"e"},
				Usage:    "Prometheus endpoint URL",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "start",
				Usage: "Start time (RFC3339 format)",
				Value: time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
			},
			&cli.StringFlag{
				Name:  "end",
				Usage: "End time (RFC3339 format)",
				Value: time.Now().Format(time.RFC3339),
			},
			&cli.DurationFlag{
				Name:  "step",
				Usage: "Format: 1s, 1m, 1h, 1d, default is 1s.",
				Value: time.Second,
			},
			&cli.StringFlag{
				Name: "query",
				Usage: "PromQL query to filter time series, e.g. use {risingwave_cluster=\"default\"} " +
					"to dump all time series with the label risingwave_cluster=default. " +
					"If not provided, all time series will be dumped.",
				Value: "",
			},
			&cli.BoolFlag{
				Name:  "plain",
				Usage: "Output in uncompressed NDJSON format",
				Value: false,
			},
			&cli.Float64Flag{
				Name:  "query-ratio",
				Usage: "(deprecated, use memory-ratio instead) (0, 1], if OOM, reduce the memory usage in Prometheus instance by this ratio",
				Value: 0,
			},
			&cli.Float64Flag{
				Name:  "memory-ratio",
				Usage: "(0, 1], if OOM, reduce the memory usage in Prometheus instance by this ratio",
				Value: 1,
			},
			&cli.IntFlag{
				Name:    "parts",
				Aliases: []string{"p"},
				Usage:   "Divide query results into multiple parts. Useful for handling large datasets and resuming from the last completed part if interrupted.",
				Value:   1,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// runDump implements the 'dump' command to dump Prometheus data to a file
func runDump(c *cli.Context) error {
	outDir := c.String("out")
	if outDir == "" {
		return fmt.Errorf("out is required")
	}
	if outDir == "." {
		// get current directory
		wd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, "failed to get current directory")
		}
		outDir = wd
	}

	endpoint := c.String("endpoint")
	if endpoint == "" {
		return fmt.Errorf("prometheus endpoint is required")
	}

	startStr := c.String("start")
	endStr := c.String("end")
	step := c.Duration("step")
	queryInterval := c.Duration("query-interval")
	parts := c.Int("parts")

	if parts < 1 {
		return fmt.Errorf("parts must be greater than 0")
	}

	// Parse memory-ratio
	var memoryRatio float64
	queryRatio := c.Float64("query-ratio")
	if queryRatio > 0 { // use query-ratio for compatibility
		memoryRatio = queryRatio
	} else { // use memory-ratio
		memoryRatio = c.Float64("memory-ratio")
		if memoryRatio < 0 || memoryRatio > 1 {
			return fmt.Errorf("memory-ratio must be between 0 and 1")
		}
	}

	// Parse time strings
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return errors.Wrap(err, "failed to parse start time")
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return errors.Wrap(err, "failed to parse end time")
	}

	fmt.Printf("Dumping Prometheus data from %s to %s\n", endpoint, outDir)
	fmt.Printf("Time range: %s to %s with step %s\n", start.Format(time.RFC3339), end.Format(time.RFC3339), step)

	// digest
	digest := sha256.New()
	digest.Write([]byte(fmt.Sprintf("%s-%s-%s-%s-%s-%v-%d", endpoint, startStr, endStr, step, c.String("query"), c.Bool("plain"), parts)))
	digestStr := hex.EncodeToString(digest.Sum(nil))[:8]

	if parts == 1 { // output to a folder
		outDir = filepath.Join(outDir, fmt.Sprintf("promdump_%s", digestStr))
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
			part, err := strconv.Atoi(strings.TrimSuffix(file.Name(), ".ndjson.gz"))
			if err != nil {
				continue
			}
			if part > maxPart {
				maxPart = part
			}
		}

		// split the start and end time into parts
		var timeRanges [][]time.Time
		rangeInterval := end.Sub(start) / time.Duration(parts)
		for i := 0; i < parts; i++ {
			timeRanges = append(timeRanges, []time.Time{
				start.Add(time.Duration(i) * rangeInterval),
				start.Add(time.Duration(i+1) * rangeInterval),
			})
		}

		for i, timeRange := range timeRanges {
			fmt.Printf("======Dumping part %d (%d/%d) %s to %s=======\n", i, i+1, parts, timeRange[0].Format(time.RFC3339), timeRange[1].Format(time.RFC3339))
			if i < maxPart {
				continue
			}
			outFile := filepath.Join(outDir, fmt.Sprintf("%d.ndjson.gz", i))

			// Create dump options
			opt := &promdump.DumpOpt{
				Endpoint:      endpoint,
				Start:         timeRange[0],
				End:           timeRange[1],
				Step:          step,
				QueryInterval: queryInterval,
				Query:         c.String("query"),
				Plain:         c.Bool("plain"),
				QueryRatio:    memoryRatio,
			}

			// Execute the dump
			err = promdump.DumpPromToFile(context.Background(), opt, outFile, true)
			if err != nil {
				return errors.Wrap(err, "failed to dump prometheus data")
			}
		}
	} else { // output to a file
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return errors.Wrap(err, "failed to create output directory")
		}
		outFile := filepath.Join(outDir, fmt.Sprintf("promdump_%s.ndjson.gz", digestStr))

		// Create dump options
		opt := &promdump.DumpOpt{
			Endpoint:      endpoint,
			Start:         start,
			End:           end,
			Step:          step,
			QueryInterval: queryInterval,
			Query:         c.String("query"),
			Plain:         c.Bool("plain"),
			QueryRatio:    memoryRatio,
		}

		// Execute the dump
		err = promdump.DumpPromToFile(context.Background(), opt, outFile, true)
		if err != nil {
			return errors.Wrap(err, "failed to dump prometheus data")
		}

		fmt.Printf("Successfully dumped prometheus data to %s\n", outFile)
	}

	return nil
}
