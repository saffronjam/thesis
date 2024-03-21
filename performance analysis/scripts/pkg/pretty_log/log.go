package pretty_log

import (
	"fmt"
	"time"
)

// TaskGroup prints the title of a group of tasks in bright blue color.
func TaskGroup(format string, a ...interface{}) {
	title := fmt.Sprintf(format, a...)
	bold := "\033[1m"
	brightBlue := "\033[94m"
	reset := "\033[0m"
	now := time.Now().Format("2006/01/02 15:04:05")
	fmt.Printf("[%s] %s%s%s%s\n", now, brightBlue, bold, title, reset)
}

// BeginTask prints the beginning of a task with its name in orange and "..." in grey, without a newline at the end.
func BeginTask(format string, a ...interface{}) {
	taskName := fmt.Sprintf(format, a...)
	orange := "\033[38;5;208m"
	grey := "\033[90m"
	reset := "\033[0m"
	now := time.Now().Format("2006/01/02 15:04:05")
	fmt.Printf("[%s] %s%s%s %s...%s ", now, orange, taskName, reset, grey, reset)
}

// EndTask prints a green checkmark, then ends the line.
func EndTask() {
	green := "\033[32m"
	reset := "\033[0m"
	fmt.Printf("%s✓%s\n", green, reset)
}

// TaskResult prints the result of a task in cyan color.
func TaskResult(format string, a ...interface{}) {
	result := fmt.Sprintf(format, a...)
	cyan := "\033[36m"
	reset := "\033[0m"
	fmt.Printf("  %s- %s%s\n", cyan, result, reset)
}
