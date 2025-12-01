package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/risingwavelabs/promdump/pkg/prompush"
	"github.com/risingwavelabs/promdump/utils"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:   "prompush",
		Usage:  "Push Prometheus data to a remote endpoint",
		Action: runPush,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "path",
				Aliases:  []string{"p"},
				Usage:    "The path of the input file or folder",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "vm-endpoint",
				Aliases: []string{"e"},
				Usage:   "VictoriaMetrics endpoint URL",
			},
			&cli.IntFlag{
				Name:     "batch-size",
				Aliases:  []string{"b"},
				Usage:    "Batch size",
				Required: false,
				Value:    1000,
			},
			&cli.BoolFlag{
				Name:     "noop",
				Usage:    "Do not actually push data, just simulate",
				Required: false,
				Value:    false,
			},
			&cli.BoolFlag{
				Name:  "amp",
				Usage: "Parse response from the Prometheus-compatible APIs of AWS Managed Prometheus",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "ignore-invalid-files",
				Usage: "Ignore invalid files and continue processing other files when a directory is provided as input",
				Value: false,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
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

func runPush(c *cli.Context) error {
	vmEndpoint := c.String("vm-endpoint")
	path := c.String("path")
	batchSize := c.Int("batch-size")
	noop := c.Bool("noop")
	amp := c.Bool("amp")
	ignoreInvalidFiles := c.Bool("ignore-invalid-files")

	if batchSize <= 0 {
		return fmt.Errorf("batch-size must be greater than 0")
	}
	if len(vmEndpoint) == 0 && !noop {
		return fmt.Errorf("vm-endpoint is required")
	}
	if len(path) == 0 {
		return fmt.Errorf("path is required")
	}

	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	files := []string{}
	if fi.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("failed to read directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			files = append(files, filepath.Join(path, entry.Name()))
		}
	} else {
		files = []string{path}
	}

	pw := prompush.NewPushWorker(c.Context, vmEndpoint, batchSize, noop)
	defer pw.Close()

	var pusher prompush.Pusher
	if amp {
		pusher = &prompush.AWSManagedPrometheusPusher{}
	} else {
		pusher = &prompush.NDJSONPusher{}
	}

	for i, filename := range files {
		fmt.Printf("\nPushing %s (%d/%d)\n", filename, i+1, len(files))

		file, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		fileInfo, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to get file info: %w", err)
		}
		fileSize := fileInfo.Size()

		var reader io.Reader
		if strings.HasSuffix(filename, ".gz") {
			gzReader, err := gzip.NewReader(file)
			if err != nil {
				return fmt.Errorf("failed to create gzip reader: %w", err)
			}
			defer gzReader.Close()
			reader = gzReader
		} else {
			reader = file
		}

		showProgress := func() error {
			// Get current position in the compressed file
			currentPos, err := file.Seek(0, io.SeekCurrent)
			if err != nil {
				return fmt.Errorf("failed to get current file position: %w", err)
			}

			// Display progress based on compressed file position
			fmt.Printf("\033[2K\rprogress: %s", utils.RenderProgressBar(float32(currentPos)/float32(fileSize)))
			return nil
		}

		if err := pusher.Push(c.Context, reader, pw, showProgress, ignoreInvalidFiles); err != nil {
			return errors.Wrap(err, "failed to push data")
		}
	}

	return pw.Flush(c.Context)
}
