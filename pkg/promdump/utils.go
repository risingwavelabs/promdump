package promdump

import "time"

type TimeRange struct {
	Start time.Time
	End   time.Time
}

// calTimeRanges calculates the time ranges for the given start and end time
func calTimeRanges(start time.Time, end time.Time, step time.Duration, memoryRatio float32) []TimeRange {
	maxDuration := time.Duration(float32(PrometheusDefaultMaxResolution)*memoryRatio) * step
	chunks := []TimeRange{}
	for {
		d := end.Sub(start)
		if d < maxDuration {
			chunks = append(chunks, TimeRange{
				Start: start,
				End:   end,
			})
			break
		}
		chunks = append(chunks, TimeRange{
			Start: start.Add(step), // last end is inclusive
			End:   start.Add(maxDuration),
		})
		start = start.Add(maxDuration)
	}
	return chunks
}
