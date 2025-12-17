package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls [path_or_id]",
	Short: "List files in a directory",
	Long:  "List files in a directory by ID or path (e.g. 'ls 3' or 'ls /Common documents/RH')",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient(cfg)

		fileID := 1
		if len(args) > 0 {
			// Try to parse as ID first
			if id, err := strconv.Atoi(args[0]); err == nil {
				fileID = id
			} else {
				// Otherwise resolve as path
				file, err := client.FindFileByPath(args[0])
				if err != nil {
					return err
				}
				fileID = file.ID
			}
		}

		files, err := client.ListFiles(fileID)
		if err != nil {
			return err
		}

		sort.Slice(files, func(i, j int) bool {
			return files[i].Name < files[j].Name
		})

		fmt.Printf("TYPE\tMODIFIED\t\tID\tNAME\n")
		for _, f := range files {
			printFile(&f)
		}
		return nil
	},
}

func printFile(f *api.File) {
	modTime := time.Unix(f.LastModifiedAt, 0).Format("2006-01-02 15:04")
	fmt.Printf("%s\t%s\t%d\t%s\n", f.Type, modTime, f.ID, f.Name)
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
