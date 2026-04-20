package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/gfaivre/ktools/internal/logging"
	"github.com/spf13/cobra"
)

var (
	reportActions  []string
	reportDepth    string
	reportFiles    []int
	reportFrom     int64
	reportUntil    int64
	reportUserID   int
	reportUsers    []int
	reportTerms    string
	reportWait     bool
	reportDownload bool
	reportOutput   string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Manage activity reports",
	Long:  "Create and list activity reports on the drive. Requires admin_token in config.",
}

var reportCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new activity report",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.AdminToken == "" {
			return fmt.Errorf("admin_token required for report (config or KTOOLS_ADMIN_TOKEN)")
		}

		ctx := cmd.Context()
		client := api.NewAdminClient(cfg)

		now := time.Now()
		from := reportFrom
		until := reportUntil
		if from == 0 {
			from = now.AddDate(0, -3, 0).Unix()
		}
		if until == 0 {
			until = now.Unix()
		}

		opts := api.ReportOptions{
			Actions: reportActions,
			Depth:   reportDepth,
			Files:   reportFiles,
			From:    from,
			Until:   until,
			UserID:  reportUserID,
			Users:   reportUsers,
			Terms:   reportTerms,
		}

		waitForReport := reportWait || reportDownload

		fmt.Fprintln(os.Stderr, "Creating report...")
		reportID, err := client.CreateReport(ctx, opts)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Report ID: %d\n", reportID)

		if !waitForReport {
			fmt.Printf("%d\n", reportID)
			return nil
		}

		fmt.Fprintln(os.Stderr, "Waiting for report to be ready...")
		const pollInterval = 3 * time.Second
		const maxWait = 5 * time.Minute

		deadline := time.Now().Add(maxWait)
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(pollInterval):
			}

			report, err := client.GetReport(ctx, reportID)
			if err != nil {
				logging.Debug("poll error", "err", err)
				continue
			}
			logging.Debug("report status", "status", report.Status)
			if report.Status == "done" || report.Status == "failed" {
				fmt.Fprintln(os.Stderr)
				exportURL := client.ReportExportURL(reportID)
				printReport(report, exportURL)
				if report.Status == "done" && reportDownload {
					return downloadReport(ctx, client, reportID, reportOutput)
				}
				return nil
			}
			fmt.Fprintf(os.Stderr, "\rStatus: %-20s", report.Status+"...")
		}

		return fmt.Errorf("report %d not ready after %s", reportID, maxWait)
	},
}

var reportListCmd = &cobra.Command{
	Use:   "list",
	Short: "List activity reports",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.AdminToken == "" {
			return fmt.Errorf("admin_token required for report (config or KTOOLS_ADMIN_TOKEN)")
		}

		ctx := cmd.Context()
		client := api.NewAdminClient(cfg)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tSIZE\tCREATED\tDOWNLOAD URL")

		page := 1
		for {
			reports, pages, err := client.ListReports(ctx, page)
			if err != nil {
				return err
			}
			for _, r := range reports {
				url := r.DownloadURL
				if url == "" {
					url = "-"
				}
				size := r.Size
				if size == "" || size == "0" {
					size = "-"
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
					r.ID,
					r.Status,
					size,
					time.Unix(r.CreatedAt, 0).Format("2006-01-02 15:04"),
					url,
				)
			}
			if page >= pages {
				break
			}
			page++
		}

		return w.Flush()
	},
}

var reportDeleteCmd = &cobra.Command{
	Use:   "delete <report_id>",
	Short: "Delete an activity report",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.AdminToken == "" {
			return fmt.Errorf("admin_token required for report (config or KTOOLS_ADMIN_TOKEN)")
		}

		reportID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid report ID: %s", args[0])
		}

		ctx := cmd.Context()
		client := api.NewAdminClient(cfg)

		if err := client.DeleteReport(ctx, reportID); err != nil {
			return err
		}

		fmt.Printf("Report %d deleted\n", reportID)
		return nil
	},
}

var reportDeleteAllCmd = &cobra.Command{
	Use:   "delete-all",
	Short: "Delete all activity reports",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.AdminToken == "" {
			return fmt.Errorf("admin_token required for report (config or KTOOLS_ADMIN_TOKEN)")
		}

		ctx := cmd.Context()
		client := api.NewAdminClient(cfg)

		deleted := 0
		seen := make(map[int]bool)
		for {
			reports, _, err := client.ListReports(ctx, 1)
			if err != nil {
				return err
			}
			if len(reports) == 0 {
				break
			}
			for _, r := range reports {
				if seen[r.ID] {
					return fmt.Errorf("report %d still present after deletion (possibly asynchronous), aborting to avoid infinite loop", r.ID)
				}
				seen[r.ID] = true
				if err := client.DeleteReport(ctx, r.ID); err != nil {
					return fmt.Errorf("failed to delete report %d: %w", r.ID, err)
				}
				deleted++
				fmt.Fprintf(os.Stderr, "\rDeleted %d reports...", deleted)
			}
		}

		fmt.Printf("\nDeleted %d reports\n", deleted)
		return nil
	},
}


func downloadReport(ctx context.Context, client *api.Client, reportID int, output string) error {
	fmt.Fprintln(os.Stderr, "Downloading report...")
	data, err := client.DownloadReport(ctx, reportID)
	if err != nil {
		return err
	}

	if output == "" {
		dir := "reports"
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create reports directory: %w", err)
		}
		output = fmt.Sprintf("%s/report_%d.csv", dir, reportID)
	}

	if err := os.WriteFile(output, data, 0600); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	fmt.Printf("Saved to: %s (%d bytes)\n", output, len(data))

	if err := client.DeleteReport(ctx, reportID); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not delete report %d: %v\n", reportID, err)
	} else {
		fmt.Fprintf(os.Stderr, "Report %d deleted from server\n", reportID)
	}

	return nil
}

func printReport(r *api.Report, exportURL string) {
	size := r.Size
	if size == "" || size == "0" {
		size = "-"
	} else {
		size += " bytes"
	}
	fmt.Printf("Status:       %s\n", r.Status)
	fmt.Printf("Size:         %s\n", size)
	fmt.Printf("Download URL: %s\n", exportURL)
	fmt.Printf("Generated by: %s <%s>\n", r.GeneratedBy.DisplayName, r.GeneratedBy.Email)
	fmt.Printf("Created at:   %s\n", time.Unix(r.CreatedAt, 0).Format("2006-01-02 15:04:05"))
}

func init() {
	reportCreateCmd.Flags().StringArrayVar(&reportActions, "action", nil, "Filter by action type (repeatable)")
	reportCreateCmd.Flags().StringVar(&reportDepth, "depth", "", "Depth: children, file, folder, unlimited")
	reportCreateCmd.Flags().IntSliceVar(&reportFiles, "file", nil, "File IDs to include (repeatable, max 500)")
	reportCreateCmd.Flags().Int64Var(&reportFrom, "from", 0, "Start timestamp (Unix)")
	reportCreateCmd.Flags().Int64Var(&reportUntil, "until", 0, "End timestamp (Unix)")
	reportCreateCmd.Flags().IntVar(&reportUserID, "user-id", 0, "Filter by single user ID")
	reportCreateCmd.Flags().IntSliceVar(&reportUsers, "user", nil, "Filter by user IDs (repeatable)")
	reportCreateCmd.Flags().StringVar(&reportTerms, "terms", "", "Search terms (min 3 chars)")
	reportCreateCmd.Flags().BoolVarP(&reportWait, "wait", "w", false, "Wait for completion and print download URL")
	reportCreateCmd.Flags().BoolVarP(&reportDownload, "download", "d", false, "Download the report after completion (implies --wait)")
	reportCreateCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "Output file path (default: report_<id>.csv)")

	reportCmd.AddCommand(reportCreateCmd)
	reportCmd.AddCommand(reportListCmd)
	reportCmd.AddCommand(reportDeleteCmd)
	reportCmd.AddCommand(reportDeleteAllCmd)
	rootCmd.AddCommand(reportCmd)
}
