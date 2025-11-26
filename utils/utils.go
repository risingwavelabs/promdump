package utils

import "fmt"

// renderProgressBar generates a string representation of a progress bar
// based on the given progress percentage (0.0 to 1.0)
func RenderProgressBar(progress float32) string {
	barWidth := 40
	completedWidth := int(float32(barWidth) * progress)

	// Build the progress bar string
	progressBar := "["
	for i := 0; i < barWidth; i++ {
		if i < completedWidth {
			progressBar += "="
		} else if i == completedWidth && progress < 1.0 {
			progressBar += ">"
		} else {
			progressBar += " "
		}
	}
	progressBar += "]"

	// Return the combined progress string with percentage
	return fmt.Sprintf("%s %.2f%%", progressBar, progress*100)
}

func TruncateString(val string, limit int) string {
	if len(val) <= limit {
		return val
	}
	return val[:limit] + "..."
}
