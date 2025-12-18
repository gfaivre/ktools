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

// formatSize formats bytes as human-readable size (KB, MB, GB)
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
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

// resolveStartPath resolves a path or ID argument to startID and startName
// Returns (1, "/") if no argument provided
func resolveStartPath(ctx context.Context, client *api.Client, arg string) (int, string, error) {
	if arg == "" {
		return 1, "/", nil
	}

	if id, err := strconv.Atoi(arg); err == nil {
		file, err := client.GetFile(ctx, id)
		if err != nil {
			return id, strconv.Itoa(id), nil // Fallback to ID as name
		}
		return id, file.Name, nil
	}

	file, err := client.FindFileByPath(ctx, arg)
	if err != nil {
		return 0, "", err
	}
	return file.ID, file.Name, nil
}
