package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/spf13/cobra"
)

type fileInfo struct {
	ID   int
	Name string
}

// resolveCategoryID resolves a category name or ID to an ID
func resolveCategoryID(client *api.Client, nameOrID string) (int, error) {
	if id, err := strconv.Atoi(nameOrID); err == nil {
		return id, nil
	}

	categories, err := client.ListCategories()
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
func getCategoryName(client *api.Client, categoryID int) string {
	categories, _ := client.ListCategories()
	for _, c := range categories {
		if c.ID == categoryID {
			return c.Name
		}
	}
	return fmt.Sprintf("%d", categoryID)
}

// collectFiles collects file IDs and names, optionally recursively
func collectFiles(client *api.Client, fileID int, recursive bool) ([]fileInfo, error) {
	rootFile, err := client.GetFile(fileID)
	if err != nil {
		return nil, err
	}

	files := []fileInfo{{ID: rootFile.ID, Name: rootFile.Name}}

	if recursive {
		children, err := client.ListFilesRecursive(fileID)
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

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage categories/tags",
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available categories",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient(cfg)

		categories, err := client.ListCategories()
		if err != nil {
			return err
		}

		for _, c := range categories {
			fmt.Printf("%d\t%s\t%s\n", c.ID, c.Color, c.Name)
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
		client := api.NewClient(cfg)

		categoryID, err := resolveCategoryID(client, args[0])
		if err != nil {
			return err
		}
		categoryName := getCategoryName(client, categoryID)

		var fileID int
		if _, err := fmt.Sscanf(args[1], "%d", &fileID); err != nil {
			return fmt.Errorf("invalid file_id: %s", args[1])
		}

		files, err := collectFiles(client, fileID, recursive)
		if err != nil {
			return err
		}

		if recursive {
			fmt.Printf("Applying [%s] to %d files...\n", categoryName, len(files))
		}

		fileIDs, fileNames := buildFileMap(files)

		const batchSize = 50
		for i := 0; i < len(fileIDs); i += batchSize {
			end := i + batchSize
			if end > len(fileIDs) {
				end = len(fileIDs)
			}

			results, err := client.AddCategoryToFiles(categoryID, fileIDs[i:end])
			if err != nil {
				return err
			}

			for _, r := range results {
				if r.Result {
					fmt.Printf("  [OK]   %s\n", fileNames[r.ID])
				} else {
					fmt.Printf("  [SKIP] %s (already tagged)\n", fileNames[r.ID])
				}
			}
		}

		fmt.Println("Done.")
		return nil
	},
}

var tagRmCmd = &cobra.Command{
	Use:   "rm <category> <file_id>",
	Short: "Remove a category from a file/directory",
	Long:  "Remove a category (name or ID) from a file/directory",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient(cfg)

		categoryID, err := resolveCategoryID(client, args[0])
		if err != nil {
			return err
		}
		categoryName := getCategoryName(client, categoryID)

		var fileID int
		if _, err := fmt.Sscanf(args[1], "%d", &fileID); err != nil {
			return fmt.Errorf("invalid file_id: %s", args[1])
		}

		files, err := collectFiles(client, fileID, recursive)
		if err != nil {
			return err
		}

		if recursive {
			fmt.Printf("Removing [%s] from %d files...\n", categoryName, len(files))
		}

		fileIDs, fileNames := buildFileMap(files)

		const batchSize = 50
		for i := 0; i < len(fileIDs); i += batchSize {
			end := i + batchSize
			if end > len(fileIDs) {
				end = len(fileIDs)
			}

			results, err := client.RemoveCategoryFromFiles(categoryID, fileIDs[i:end])
			if err != nil {
				return err
			}

			for _, r := range results {
				if r.Result {
					fmt.Printf("  [OK]   %s\n", fileNames[r.ID])
				} else {
					fmt.Printf("  [SKIP] %s (not tagged)\n", fileNames[r.ID])
				}
			}
		}

		fmt.Println("Done.")
		return nil
	},
}

func init() {
	tagAddCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Apply recursively to all children")
	tagRmCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Remove recursively from all children")

	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRmCmd)
	rootCmd.AddCommand(tagCmd)
}
