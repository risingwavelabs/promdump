package promdump

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/prometheus/prometheus/promql/parser"
)

type GrafanaDashboardParser struct {
	set map[string]struct{}
}

func NewGrafanaDashboardParser() *GrafanaDashboardParser {
	return &GrafanaDashboardParser{
		set: make(map[string]struct{}),
	}
}

type Dashboard struct {
	Panels []Panel `json:"panels"`
}

type Panel struct {
	Panels  []Panel  `json:"panels"`
	Targets []Target `json:"targets"`
}

type Target struct {
	Query string `json:"expr"`
}

func (p *GrafanaDashboardParser) Parse(dashboard string) ([]string, error) {
	var (
		content []byte
		err     error
	)
	if isFilePath(dashboard) {
		content, err = readFileContent(dashboard)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read grafana dashboard from file %s, please specify a valid version or a file path", dashboard)
		}
	} else {
		content, err = readFromVersion(dashboard)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read grafana dashboard from version %s, please specify a valid version or a file path", dashboard)
		}
	}
	var board Dashboard
	if err := json.Unmarshal(content, &board); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal grafana dashboard JSON")
	}
	for _, panel := range board.Panels {
		if err := p.fetchMetricsNamesFromPanel(&panel); err != nil {
			return nil, err
		}
	}
	var metrics []string
	for metric := range p.set {
		metrics = append(metrics, metric)
	}

	return metrics, nil
}

func cleanQuery(s string) string {
	s = strings.ReplaceAll(s, "$__interval", "5m")
	s = strings.ReplaceAll(s, "$__rate_interval", "5m")
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune("$", r) {
			return -1
		}
		return r
	}, s)
	return s
}

func (p *GrafanaDashboardParser) fetchMetricsNamesFromPanel(panel *Panel) error {
	for _, target := range panel.Targets {
		if target.Query == "" {
			continue
		}
		expr, err := parser.ParseExpr(cleanQuery(target.Query))
		if err != nil {
			return errors.Wrapf(err, "failed to parse PromQL expression: %s", target.Query)
		}
		p.extractMetrics(expr)
	}

	for _, panel := range panel.Panels {
		if err := p.fetchMetricsNamesFromPanel(&panel); err != nil {
			return err
		}
	}
	return nil
}

func (p *GrafanaDashboardParser) extractMetrics(expr parser.Expr) {
	parser.Inspect(expr, func(n parser.Node, _ []parser.Node) error {
		switch x := n.(type) {
		case *parser.VectorSelector:
			p.set[x.Name] = struct{}{}
		case *parser.MatrixSelector:
			if vs, ok := x.VectorSelector.(*parser.VectorSelector); ok {
				p.set[vs.Name] = struct{}{}
			}
		}
		return nil
	})
}

func readFileContent(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file content")
	}
	return content, nil
}

func readFromVersion(version string) ([]byte, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/risingwavelabs/risingwave/refs/tags/%s/grafana/risingwave-dev-dashboard.json", version)
	return readFromURL(url)
}

func readFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch grafana dashboard from URL")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch grafana dashboard: status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	return body, nil
}

func isFilePath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
