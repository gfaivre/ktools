package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gfaivre/ktools/internal/api"
	"github.com/gfaivre/ktools/internal/logging"
	"github.com/spf13/cobra"
)

var (
	activitiesLimit    int
	activitiesAll      bool
	activitiesWithTags bool
	activitiesAsc      bool
	activitiesActions  []string
	activitiesFrom     int64
	activitiesUntil    int64
	activitiesUsers    []int
)

var activitiesCmd = &cobra.Command{
	Use:   "activities",
	Short: "List drive activity log",
	Long:  "Fetch and display the activity log for the drive (most recent first by default). Requires admin_token in config.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.AdminToken == "" {
			return fmt.Errorf("admin_token required for activities (config or KTOOLS_ADMIN_TOKEN)")
		}

		ctx := cmd.Context()
		client := api.NewAdminClient(cfg)

		order := "desc"
		if activitiesAsc {
			order = "asc"
		}

		opts := api.ActivitiesOptions{
			Limit:   activitiesLimit,
			Order:   order,
			Actions: activitiesActions,
			From:    activitiesFrom,
			Until:   activitiesUntil,
			Users:   activitiesUsers,
		}

		var collected []api.Activity
		for {
			activities, nextCursor, hasMore, err := client.ListActivities(ctx, opts)
			if err != nil {
				return err
			}
			collected = append(collected, activities...)
			if !hasMore || !activitiesAll {
				break
			}
			opts.Cursor = nextCursor
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if activitiesWithTags {
			fmt.Fprintln(w, "DATE\tACTION\tUSER\tPATH\tTAGS\tID")
		} else {
			fmt.Fprintln(w, "DATE\tACTION\tUSER\tPATH\tID")
		}

		for _, a := range collected {
			t := time.Unix(a.CreatedAt, 0).Format("2006-01-02 15:04:05")

			user := "-"
			if a.User != nil {
				user = a.User.DisplayName
			}

			path := a.NewPath
			if path == "" {
				path = a.OldPath
			}
			if path == "" {
				path = "-"
			}

			if activitiesWithTags {
				tags := "-"
				if a.FileID > 0 {
					categories, err := client.GetFileCategories(ctx, a.FileID)
					if err != nil {
						logging.Debug("failed to fetch categories", "file_id", a.FileID, "err", err)
						tags = "?"
					} else if len(categories) > 0 {
						names := make([]string, len(categories))
						for i, c := range categories {
							names[i] = c.Name
						}
						tags = strings.Join(names, ", ")
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n", t, a.Action, user, path, tags, a.ID)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", t, a.Action, user, path, a.ID)
			}
		}

		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Printf("\nTotal: %d activities\n", len(collected))
		return nil
	},
}

func init() {
	activitiesCmd.Flags().IntVarP(&activitiesLimit, "limit", "n", 50, "Number of activities per page (max 1000)")
	activitiesCmd.Flags().BoolVarP(&activitiesAll, "all", "a", false, "Fetch all pages")
	activitiesCmd.Flags().BoolVar(&activitiesWithTags, "with-tags", false, "Enrich file activities with tags (1 extra API call per file)")
	activitiesCmd.Flags().BoolVar(&activitiesAsc, "asc", false, "Sort ascending (oldest first)")
	activitiesCmd.Flags().StringArrayVar(&activitiesActions, "action", nil, "Filter by action (repeatable, e.g. --action file_trash --action file_delete)")
	activitiesCmd.Flags().Int64Var(&activitiesFrom, "from", 0, "Filter from timestamp (Unix)")
	activitiesCmd.Flags().Int64Var(&activitiesUntil, "until", 0, "Filter until timestamp (Unix)")
	activitiesCmd.Flags().IntSliceVar(&activitiesUsers, "user", nil, "Filter by user ID (repeatable)")
	rootCmd.AddCommand(activitiesCmd)
}
