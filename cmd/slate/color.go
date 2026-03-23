package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/e1sidy/slate"
	"golang.org/x/term"
)

const (
	timeFormatFull  = "2006-01-02 15:04"
	timeFormatShort = "01-02 15:04"
)

const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
)

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func colorize(color, s string) string {
	if !colorEnabled() {
		return s
	}
	return color + s + reset
}

func statusIcon(s slate.Status) string {
	switch s {
	case slate.StatusOpen:
		return " "
	case slate.StatusInProgress:
		return ">"
	case slate.StatusBlocked:
		return "!"
	case slate.StatusDeferred:
		return "~"
	case slate.StatusClosed:
		return "x"
	case slate.StatusCancelled:
		return "-"
	default:
		return "?"
	}
}

func colorStatus(s slate.Status) string {
	icon := statusIcon(s)
	bracket := "[" + icon + "]"
	if !colorEnabled() {
		return bracket
	}
	switch s {
	case slate.StatusInProgress:
		return colorize(blue+bold, bracket)
	case slate.StatusBlocked:
		return colorize(red+bold, bracket)
	case slate.StatusDeferred:
		return colorize(yellow, bracket)
	case slate.StatusClosed:
		return colorize(green, bracket)
	case slate.StatusCancelled:
		return colorize(magenta, bracket)
	default:
		return bracket
	}
}

func colorPriority(p slate.Priority) string {
	label := fmt.Sprintf("P%d", p)
	if !colorEnabled() {
		return label
	}
	switch p {
	case slate.P0:
		return colorize(red+bold, label)
	case slate.P1:
		return colorize(yellow+bold, label)
	case slate.P3, slate.P4:
		return colorize(dim, label)
	default:
		return label
	}
}

func colorID(id string) string {
	return colorize(dim, id)
}

func colorAssignee(a string) string {
	if a == "" {
		return ""
	}
	return colorize(cyan, "@"+a)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
