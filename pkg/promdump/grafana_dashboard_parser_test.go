package promdump

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrafanaDashboardParser(t *testing.T) {
	p := NewGrafanaDashboardParser()
	metrics, err := p.Parse("v2.6.2")
	require.NoError(t, err)

	fmt.Println(len(metrics))
}
