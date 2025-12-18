package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

type fileInfo struct {
	ID   int
	Name string
}

// resolveCategoryID resolves a category name or ID to an ID
func resolveCategoryID(ctx context.Context, client *api.Client, nameOrID string) (int, error) {
	if id, err := strconv.Atoi(nameOrID); err == nil {
		return id, nil
	}

	categories, err := client.ListCategories(ctx)
	if err != nil {
		return 0, err
	}

	nameOrID = strings.ToLower(nameOrID)
	for _, c := range categories {
		if strings.ToLower(c.Name) == nameOrID {
			return c.ID, nil
		}
	}

	return 0, fmt.Errorf("category '%s' not found", nameOrID)
}

// getCategoryName returns the category name for a given ID
func getCategoryName(ctx context.Context, client *api.Client, categoryID int) string {
	categories, _ := client.ListCategories(ctx)
	for _, c := range categories {
		if c.ID == categoryID {
			return c.Name
		}
	}
	return fmt.Sprintf("%d", categoryID)
}

// collectFiles collects file IDs and names, optionally recursively
func collectFiles(ctx context.Context, client *api.Client, fileID int, recursive bool) ([]fileInfo, error) {
	rootFile, err := client.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	files := []fileInfo{{ID: rootFile.ID, Name: rootFile.Name}}

	if recursive {
		// Show scanning progress
		progress := func(dirName string, fileCount int) {
			fmt.Fprintf(os.Stderr, "\r\033[KScanning: %s (%d files found)", truncateName(dirName, 40), fileCount)
		}

		children, err := client.ListFilesRecursiveWithProgress(ctx, fileID, progress)
		fmt.Fprintln(os.Stderr) // Clear line
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
	Use:   "add <category> <file_id>",
	Short: "Add a category to a file/directory",
	Long:  "Add a category (name or ID) to a file/directory",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := api.NewClient(cfg)

		categoryID, err := resolveCategoryID(ctx, client, args[0])
		if err != nil {
			return err
		}
		categoryName := getCategoryName(ctx, client, categoryID)

		var fileID int
		if _, err := fmt.Sscanf(args[1], "%d", &fileID); err != nil {
			return fmt.Errorf("invalid file_id: %s", args[1])
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
	Use:   "rm <category> <file_id>",
	Short: "Remove a category from a file/directory",
	Long:  "Remove a category (name or ID) from a file/directory",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := api.NewClient(cfg)

		categoryID, err := resolveCategoryID(ctx, client, args[0])
		if err != nil {
			return err
		}
		categoryName := getCategoryName(ctx, client, categoryID)

		var fileID int
		if _, err := fmt.Sscanf(args[1], "%d", &fileID); err != nil {
			return fmt.Errorf("invalid file_id: %s", args[1])
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
