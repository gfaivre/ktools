package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/gfaivre/ktools/internal/logging"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

type fileInfo struct {
	ID   int
	Name string
}

// resolveCategory resolves a category name or ID to both ID and name
func resolveCategory(ctx context.Context, client *api.Client, nameOrID string) (int, string, error) {
	// If it's a numeric ID, fetch categories to get the name
	if id, err := strconv.Atoi(nameOrID); err == nil {
		categories, err := client.ListCategories(ctx)
		if err != nil {
			return id, strconv.Itoa(id), nil // Fallback to ID as name
		}
		for _, c := range categories {
			if c.ID == id {
				return id, c.Name, nil
			}
		}
		return id, strconv.Itoa(id), nil // ID not found, use ID as name
	}

	// It's a name, fetch categories to get the ID
	categories, err := client.ListCategories(ctx)
	if err != nil {
		return 0, "", err
	}

	nameLower := strings.ToLower(nameOrID)
	for _, c := range categories {
		if strings.ToLower(c.Name) == nameLower {
			return c.ID, c.Name, nil
		}
	}

	return 0, "", fmt.Errorf("category '%s' not found", nameOrID)
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

// collectFiles collects file IDs and names, optionally recursively
func collectFiles(ctx context.Context, client *api.Client, fileID int, recursive bool) ([]fileInfo, error) {
	logging.Debug("collecting files", "fileID", fileID, "recursive", recursive)
	rootFile, err := client.GetFile(ctx, fileID)
	if err != nil {
		logging.Debug("GetFile error", "err", err)
		return nil, err
	}

	files := []fileInfo{{ID: rootFile.ID, Name: rootFile.Name}}

	if recursive {
		// Show scanning progress
		progress := func(dirName string, fileCount int) {
			fmt.Fprintf(os.Stderr, "\r\033[KScanning: %s (%d files found)", truncateName(dirName, 40), fileCount)
		}

		logging.Debug("starting recursive scan", "fileID", fileID)
		children, err := client.ListFilesRecursiveWithProgress(ctx, fileID, rootFile.Name, progress)
		fmt.Fprintln(os.Stderr) // Clear line
		logging.Debug("recursive scan completed", "count", len(children), "err", err)
		if err != nil {
			return nil, err
		}
		for _, f := range children {
			files = append(files, fileInfo{ID: f.ID, Name: f.Name})
		}
	}

	return files, nil
}

// buildFileMap creates ID slice and ID->name map from file list
func buildFileMap(files []fileInfo) ([]int, map[int]string) {
	fileNames := make(map[int]string, len(files))
	fileIDs := make([]int, 0, len(files))
	for _, f := range files {
		fileIDs = append(fileIDs, f.ID)
		fileNames[f.ID] = f.Name
	}
	return fileIDs, fileNames
}

// newProgressBar creates a progress bar for file operations
func newProgressBar(total int, description string) *progressbar.ProgressBar {
	return progressbar.NewOptions(total,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionClearOnFinish(),
	)
}

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage categories/tags",
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available categories",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := api.NewClient(cfg)

		categories, err := client.ListCategories(ctx)
		if err != nil {
			return err
		}

		for _, c := range categories {
			fmt.Printf("%d\t%s %s\t%s\n", c.ID, hexToANSI(c.Color), c.Color, c.Name)
		}
		return nil
	},
}

var recursive bool

var tagAddCmd = &cobra.Command{
	Use:   "add <category> <file_or_path>",
	Short: "Add a category to a file/directory",
	Long:  "Add a category (name or ID) to a file/directory (by ID or path)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := api.NewClient(cfg)

		categoryID, categoryName, err := resolveCategory(ctx, client, args[0])
		if err != nil {
			return err
		}

		fileID, err := resolveFileID(ctx, client, args[1])
		if err != nil {
			return err
		}

		files, err := collectFiles(ctx, client, fileID, recursive)
		if err != nil {
			return err
		}

		fileIDs, fileNames := buildFileMap(files)

		bar := newProgressBar(len(files), fmt.Sprintf("Adding [%s]", categoryName))
		var okCount, skipCount int

		const batchSize = 50
		for i := 0; i < len(fileIDs); i += batchSize {
			end := i + batchSize
			if end > len(fileIDs) {
				end = len(fileIDs)
			}

			results, err := client.AddCategoryToFiles(ctx, categoryID, fileIDs[i:end])
			if err != nil {
				fmt.Fprintln(os.Stderr)
				return err
			}

			for _, r := range results {
				name := fileNames[r.ID]
				bar.Describe(truncateName(name, 30))
				bar.Add(1)
				if r.Result {
					okCount++
				} else {
					skipCount++
				}
			}
		}

		fmt.Fprintf(os.Stderr, "\nDone: %d tagged, %d skipped (already tagged)\n", okCount, skipCount)
		return nil
	},
}

var tagRmCmd = &cobra.Command{
	Use:   "rm <category> <file_or_path>",
	Short: "Remove a category from a file/directory",
	Long:  "Remove a category (name or ID) from a file/directory (by ID or path)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := api.NewClient(cfg)

		categoryID, categoryName, err := resolveCategory(ctx, client, args[0])
		if err != nil {
			return err
		}

		fileID, err := resolveFileID(ctx, client, args[1])
		if err != nil {
			return err
		}

		files, err := collectFiles(ctx, client, fileID, recursive)
		if err != nil {
			return err
		}

		fileIDs, fileNames := buildFileMap(files)

		bar := newProgressBar(len(files), fmt.Sprintf("Removing [%s]", categoryName))
		var okCount, skipCount int

		const batchSize = 50
		for i := 0; i < len(fileIDs); i += batchSize {
			end := i + batchSize
			if end > len(fileIDs) {
				end = len(fileIDs)
			}

			results, err := client.RemoveCategoryFromFiles(ctx, categoryID, fileIDs[i:end])
			if err != nil {
				fmt.Fprintln(os.Stderr)
				return err
			}

			for _, r := range results {
				name := fileNames[r.ID]
				bar.Describe(truncateName(name, 30))
				bar.Add(1)
				if r.Result {
					okCount++
				} else {
					skipCount++
				}
			}
		}

		fmt.Fprintf(os.Stderr, "\nDone: %d untagged, %d skipped (not tagged)\n", okCount, skipCount)
		return nil
	},
}

// truncateName truncates a filename to max length with ellipsis
func truncateName(name string, max int) string {
	if len(name) <= max {
		return name
	}
	return name[:max-3] + "..."
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

func init() {
	tagAddCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Apply recursively to all children")
	tagRmCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Remove recursively from all children")

	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRmCmd)
	rootCmd.AddCommand(tagCmd)
}
