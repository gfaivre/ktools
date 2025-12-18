package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/gfaivre/ktools/internal/logging"
	"github.com/spf13/cobra"
)

var (
	scanTop       int
	scanThreshold int
	scanAll       bool
	scanSort      string
)

// dirStats holds statistics for a directory
type dirStats struct {
	ID        int
	Name      string
	FileCount int
	Size      int64
	Depth     int
}

var scanCmd = &cobra.Command{
	Use:   "scan [path_or_id]",
	Short: "Find directories with many files",
	Long:  "Scan directories and report those containing many files (direct children only)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := api.NewClient(cfg)

		// Resolve starting point
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		startID, startName, err := resolveStartPath(ctx, client, arg)
		if err != nil {
			return err
		}

		logging.Debug("starting scan", "startID", startID, "startName", startName)

		// Collect directory stats during scan
		stats := make(map[int]*dirStats)

		progress := func(dirName string, fileCount int) {
			fmt.Fprintf(os.Stderr, "\r\033[KScanning: %s (%d files found)", truncateName(dirName, 40), fileCount)
		}

		files, err := client.ListFilesRecursiveWithProgress(ctx, startID, startName, progress)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}

		logging.Debug("scan completed", "totalFiles", len(files))

		// Build stats: count files per parent directory
		// First pass: register all directories
		for _, f := range files {
			if f.Type == "dir" {
				stats[f.ID] = &dirStats{
					ID:    f.ID,
					Name:  f.Name,
					Depth: f.Depth,
				}
			}
		}

		// Add root directory
		stats[startID] = &dirStats{
			ID:   startID,
			Name: startName,
		}

		// Second pass: count files and size per parent
		var totalFiles int
		var totalDirs int
		var totalSize int64
		for _, f := range files {
			if f.Type == "dir" {
				totalDirs++
			} else {
				totalFiles++
				totalSize += f.Size
				if parent, ok := stats[f.ParentID]; ok {
					parent.FileCount++
					parent.Size += f.Size
				}
			}
		}

		// Convert to slice (only dirs with files)
		var results []dirStats
		for _, s := range stats {
			if s.FileCount > 0 {
				results = append(results, *s)
			}
		}

		// Sort results
		switch scanSort {
		case "size":
			sort.Slice(results, func(i, j int) bool {
				return results[i].Size > results[j].Size
			})
		default: // "files"
			sort.Slice(results, func(i, j int) bool {
				return results[i].FileCount > results[j].FileCount
			})
		}

		// Filter results
		var filtered []dirStats
		if scanAll {
			// Show all directories
			filtered = results
		} else {
			// Filter by threshold
			for _, r := range results {
				if r.FileCount >= scanThreshold {
					filtered = append(filtered, r)
				}
			}

			// If nothing passes threshold, show top N anyway
			if len(filtered) == 0 && len(results) > 0 {
				if scanTop > 0 && len(results) > scanTop {
					filtered = results[:scanTop]
				} else {
					filtered = results
				}
				fmt.Fprintf(os.Stderr, "No directories with >= %d files, showing top %d:\n", scanThreshold, len(filtered))
			} else {
				// Limit to top N
				if scanTop > 0 && len(filtered) > scanTop {
					filtered = filtered[:scanTop]
				}
			}
		}

		// Output
		if len(filtered) == 0 {
			fmt.Println("No directories found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "FILES\tSIZE\t%\tID\tNAME")
		for _, r := range filtered {
			var pct float64
			if totalSize > 0 {
				pct = float64(r.Size) / float64(totalSize) * 100
			}
			fmt.Fprintf(w, "%d\t%s\t%.1f%%\t%d\t%s\n",
				r.FileCount, formatSize(r.Size), pct, r.ID, r.Name)
		}
		w.Flush()

		fmt.Printf("\nTotal: %d files, %d directories, %s\n", totalFiles, totalDirs, formatSize(totalSize))

		return nil
	},
}


func init() {
	scanCmd.Flags().IntVarP(&scanTop, "top", "n", 10, "Show top N directories (0 = unlimited)")
	scanCmd.Flags().IntVarP(&scanThreshold, "threshold", "t", 100, "Minimum file count threshold")
	scanCmd.Flags().BoolVarP(&scanAll, "all", "a", false, "Show all directories (no filtering)")
	scanCmd.Flags().StringVarP(&scanSort, "sort", "s", "size", "Sort by: size, files")
	rootCmd.AddCommand(scanCmd)
}
