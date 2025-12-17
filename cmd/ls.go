package cmd

import (
	"fmt"
	"time"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls [file_id]",
	Short: "Liste les fichiers d'un répertoire",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient(cfg)

		// Par défaut, root du drive (ID 1)
		fileID := 1
		if len(args) > 0 {
			if _, err := fmt.Sscanf(args[0], "%d", &fileID); err != nil {
				return fmt.Errorf("file_id invalide: %s", args[0])
			}
		}

		files, err := client.ListFiles(fileID)
		if err != nil {
			return err
		}

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
