package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gfaivre/ktools/internal/api"
)

// truncateName truncates a string to max length with ellipsis
func truncateName(name string, max int) string {
	if len(name) <= max {
		return name
	}
	return name[:max-3] + "..."
}

// formatSize formats bytes as human-readable size (Ko, Mo, Go)
func formatSize(bytes int64) string {
	const (
		Ko = 1024
		Mo = Ko * 1024
		Go = Mo * 1024
	)

	switch {
	case bytes >= Go:
		return fmt.Sprintf("%.1f Go", float64(bytes)/Go)
	case bytes >= Mo:
		return fmt.Sprintf("%.1f Mo", float64(bytes)/Mo)
	case bytes >= Ko:
		return fmt.Sprintf("%.1f Ko", float64(bytes)/Ko)
	default:
		return fmt.Sprintf("%d o", bytes)
	}
}

// hexToANSI converts a hex color code to ANSI truecolor escape sequence
func hexToANSI(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return ""
	}

	r, err1 := strconv.ParseInt(hex[0:2], 16, 64)
	g, err2 := strconv.ParseInt(hex[2:4], 16, 64)
	b, err3 := strconv.ParseInt(hex[4:6], 16, 64)

	if err1 != nil || err2 != nil || err3 != nil {
		return ""
	}

	return fmt.Sprintf("\033[48;2;%d;%d;%dm  \033[0m", r, g, b)
}

// resolveFileID resolves a file ID or path to an ID
func resolveFileID(ctx context.Context, client *api.Client, idOrPath string) (int, error) {
	if id, err := strconv.Atoi(idOrPath); err == nil {
		return id, nil
	}
	file, err := client.FindFileByPath(ctx, idOrPath)
	if err != nil {
		return 0, err
	}
	return file.ID, nil
}
