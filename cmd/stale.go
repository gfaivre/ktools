package cmd

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/gfaivre/ktools/internal/logging"
	"github.com/spf13/cobra"
)

var (
	staleAge     string
	staleTop     int
	staleMinSize int64
)

// ageBucket represents an age distribution bucket
type ageBucket struct {
	label    string
	minDays  int
	maxDays  int // -1 for unlimited
	count    int
	size     int64
}

// staleFile holds file info for stale report
type staleFile struct {
	ID         int
	Name       string
	Size       int64
	ModifiedAt time.Time
	AgeDays    int
}

var staleCmd = &cobra.Command{
	Use:   "stale [path_or_id]",
	Short: "Find old files for retention review",
	Long:  "Scan directories and report files not modified since a given period (default: 2 years)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := api.NewClient(cfg)

		// Parse age threshold
		thresholdDays, err := parseAge(staleAge)
		if err != nil {
			return fmt.Errorf("invalid age format: %w", err)
		}

		// Resolve starting point
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		startID, startName, err := resolveStartPath(ctx, client, arg)
		if err != nil {
			return err
		}

		logging.Debug("starting stale scan", "startID", startID, "thresholdDays", thresholdDays)

		// Scan files
		progress := func(dirName string, fileCount int) {
			fmt.Fprintf(os.Stderr, "\r\033[KScanning: %s (%d files found)", truncateName(dirName, 40), fileCount)
		}

		files, err := client.ListFilesRecursiveWithProgress(ctx, startID, startName, progress)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}

		now := time.Now()
		thresholdDate := now.AddDate(0, 0, -thresholdDays)

		// Define age buckets
		buckets := []ageBucket{
			{label: "< 6 mois", minDays: 0, maxDays: 182},
			{label: "6m - 1 an", minDays: 182, maxDays: 365},
			{label: "1 - 2 ans", minDays: 365, maxDays: 730},
			{label: "2 - 3 ans", minDays: 730, maxDays: 1095},
			{label: "3 - 5 ans", minDays: 1095, maxDays: 1825},
			{label: "> 5 ans", minDays: 1825, maxDays: -1},
		}

		// Collect stale files and build distribution
		var staleFiles []staleFile
		var totalFiles int
		var totalSize int64

		for _, f := range files {
			if f.Type == "dir" {
				continue
			}

			totalFiles++
			totalSize += f.Size

			modTime := time.Unix(f.LastModifiedAt, 0)
			ageDays := int(now.Sub(modTime).Hours() / 24)

			// Update bucket distribution
			for i := range buckets {
				if ageDays >= buckets[i].minDays && (buckets[i].maxDays == -1 || ageDays < buckets[i].maxDays) {
					buckets[i].count++
					buckets[i].size += f.Size
					break
				}
			}

			// Collect files older than threshold
			if modTime.Before(thresholdDate) {
				if staleMinSize > 0 && f.Size < staleMinSize {
					continue
				}
				staleFiles = append(staleFiles, staleFile{
					ID:         f.ID,
					Name:       f.Name,
					Size:       f.Size,
					ModifiedAt: modTime,
					AgeDays:    ageDays,
				})
			}
		}

		// Sort stale files by size (largest first)
		sort.Slice(staleFiles, func(i, j int) bool {
			return staleFiles[i].Size > staleFiles[j].Size
		})

		// Print age distribution
		fmt.Println("Distribution par ancienneté :")
		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TRANCHE\tFICHIERS\t%\tTAILLE\t%")
		for _, b := range buckets {
			pctCount := float64(b.count) / float64(totalFiles) * 100
			pctSize := float64(b.size) / float64(totalSize) * 100
			if totalFiles == 0 {
				pctCount = 0
			}
			if totalSize == 0 {
				pctSize = 0
			}
			fmt.Fprintf(w, "%s\t%d\t%.1f%%\t%s\t%.1f%%\n",
				b.label, b.count, pctCount, formatSize(b.size), pctSize)
		}
		w.Flush()

		// Print stale files
		fmt.Println()
		fmt.Printf("Fichiers non modifiés depuis %s :\n", staleAge)
		fmt.Println()

		if len(staleFiles) == 0 {
			fmt.Println("Aucun fichier trouvé")
			return nil
		}

		// Limit output
		displayed := staleFiles
		if staleTop > 0 && len(displayed) > staleTop {
			displayed = displayed[:staleTop]
		}

		var staleSize int64
		for _, f := range staleFiles {
			staleSize += f.Size
		}

		w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "AGE\tSIZE\tMODIFIED\tID\tNAME")
		for _, f := range displayed {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
				formatAgeDays(f.AgeDays),
				formatSize(f.Size),
				f.ModifiedAt.Format("2006-01-02"),
				f.ID,
				f.Name)
		}
		w.Flush()

		if len(staleFiles) > len(displayed) {
			fmt.Printf("\n... et %d autres fichiers\n", len(staleFiles)-len(displayed))
		}

		fmt.Printf("\nTotal : %d fichiers, %s (sur %d fichiers, %s)\n",
			len(staleFiles), formatSize(staleSize), totalFiles, formatSize(totalSize))

		return nil
	},
}

// parseAge parses age string like "2y", "6m", "90d" into days
func parseAge(age string) (int, error) {
	re := regexp.MustCompile(`^(\d+)([ymadj]?)$`)
	matches := re.FindStringSubmatch(age)
	if matches == nil {
		return 0, fmt.Errorf("format: <number>[y|m|d] (ex: 2y, 6m, 90d)")
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]
	if unit == "" {
		unit = "y" // default to years
	}

	switch unit {
	case "y", "a": // years (y or a for "ans")
		return value * 365, nil
	case "m": // months
		return value * 30, nil
	case "d", "j": // days (d or j for "jours")
		return value, nil
	default:
		return value, nil
	}
}

// formatAgeDays formats age in days as human-readable string
func formatAgeDays(days int) string {
	years := days / 365
	months := (days % 365) / 30

	if years > 0 {
		if months > 0 {
			return fmt.Sprintf("%da %dm", years, months)
		}
		return fmt.Sprintf("%da", years)
	}
	if months > 0 {
		return fmt.Sprintf("%dm", months)
	}
	return fmt.Sprintf("%dj", days)
}

func init() {
	staleCmd.Flags().StringVarP(&staleAge, "age", "a", "2y", "Minimum age threshold (e.g., 2y, 6m, 90d)")
	staleCmd.Flags().IntVarP(&staleTop, "top", "n", 20, "Show top N files (0 = unlimited)")
	staleCmd.Flags().Int64VarP(&staleMinSize, "min-size", "m", 0, "Minimum file size in bytes")
	rootCmd.AddCommand(staleCmd)
}
