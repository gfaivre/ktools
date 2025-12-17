package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/spf13/cobra"
)

// resolveCategoryID resolves a category name or ID to an ID
func resolveCategoryID(client *api.Client, nameOrID string) (int, error) {
	// Try to parse as ID first
	if id, err := strconv.Atoi(nameOrID); err == nil {
		return id, nil
	}

	// Otherwise search by name
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

		categoryName := args[0]
		categoryID, err := resolveCategoryID(client, categoryName)
		if err != nil {
			return err
		}

		// Get category name if ID was passed
		categories, _ := client.ListCategories()
		for _, c := range categories {
			if c.ID == categoryID {
				categoryName = c.Name
				break
			}
		}

		var fileID int
		if _, err := fmt.Sscanf(args[1], "%d", &fileID); err != nil {
			return fmt.Errorf("invalid file_id: %s", args[1])
		}

		// Build file list with names
		type fileInfo struct {
			ID   int
			Name string
		}
		var files []fileInfo

		if recursive {
			// Get root folder first
			rootFile, err := client.GetFile(fileID)
			if err != nil {
				return err
			}
			files = append(files, fileInfo{ID: rootFile.ID, Name: rootFile.Name})

			// Then all children
			children, err := client.ListFilesRecursive(fileID)
			if err != nil {
				return err
			}
			for _, f := range children {
				files = append(files, fileInfo{ID: f.ID, Name: f.Name})
			}
			fmt.Printf("Applying [%s] to %d files...\n", categoryName, len(files))
		} else {
			rootFile, err := client.GetFile(fileID)
			if err != nil {
				return err
			}
			files = append(files, fileInfo{ID: rootFile.ID, Name: rootFile.Name})
		}

		// Create ID -> name map for display
		fileNames := make(map[int]string)
		fileIDs := make([]int, 0, len(files))
		for _, f := range files {
			fileIDs = append(fileIDs, f.ID)
			fileNames[f.ID] = f.Name
		}

		// Batch by 50 to avoid timeouts
		batchSize := 50
		for i := 0; i < len(fileIDs); i += batchSize {
			end := i + batchSize
			if end > len(fileIDs) {
				end = len(fileIDs)
			}
			batch := fileIDs[i:end]

			results, err := client.AddCategoryToFiles(categoryID, batch)
			if err != nil {
				return err
			}

			for _, r := range results {
				name := fileNames[r.ID]
				if r.Result {
					fmt.Printf("  [OK]   %s\n", name)
				} else {
					fmt.Printf("  [SKIP] %s (already tagged)\n", name)
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

		categoryName := args[0]
		categoryID, err := resolveCategoryID(client, categoryName)
		if err != nil {
			return err
		}

		// Get category name if ID was passed
		categories, _ := client.ListCategories()
		for _, c := range categories {
			if c.ID == categoryID {
				categoryName = c.Name
				break
			}
		}

		var fileID int
		if _, err := fmt.Sscanf(args[1], "%d", &fileID); err != nil {
			return fmt.Errorf("invalid file_id: %s", args[1])
		}

		// Build file list with names
		type fileInfo struct {
			ID   int
			Name string
		}
		var files []fileInfo

		if recursive {
			rootFile, err := client.GetFile(fileID)
			if err != nil {
				return err
			}
			files = append(files, fileInfo{ID: rootFile.ID, Name: rootFile.Name})

			children, err := client.ListFilesRecursive(fileID)
			if err != nil {
				return err
			}
			for _, f := range children {
				files = append(files, fileInfo{ID: f.ID, Name: f.Name})
			}
			fmt.Printf("Removing [%s] from %d files...\n", categoryName, len(files))
		} else {
			rootFile, err := client.GetFile(fileID)
			if err != nil {
				return err
			}
			files = append(files, fileInfo{ID: rootFile.ID, Name: rootFile.Name})
		}

		// Create ID -> name map for display
		fileNames := make(map[int]string)
		fileIDs := make([]int, 0, len(files))
		for _, f := range files {
			fileIDs = append(fileIDs, f.ID)
			fileNames[f.ID] = f.Name
		}

		// Batch by 50
		batchSize := 50
		for i := 0; i < len(fileIDs); i += batchSize {
			end := i + batchSize
			if end > len(fileIDs) {
				end = len(fileIDs)
			}
			batch := fileIDs[i:end]

			results, err := client.RemoveCategoryFromFiles(categoryID, batch)
			if err != nil {
				return err
			}

			for _, r := range results {
				name := fileNames[r.ID]
				if r.Result {
					fmt.Printf("  [OK]   %s\n", name)
				} else {
					fmt.Printf("  [SKIP] %s (not tagged)\n", name)
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
