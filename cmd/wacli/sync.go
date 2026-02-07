package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	appPkg "github.com/steipete/wacli/internal/app"
	"github.com/steipete/wacli/internal/logging"
	"github.com/steipete/wacli/internal/out"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var once bool
	var follow bool
	var idleExit time.Duration
	var downloadMedia bool
	var refreshContacts bool
	var refreshGroups bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync messages (requires prior auth; never shows QR)",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logging.WithComponent("sync")
			log.Info().
				Bool("once", once).
				Bool("follow", follow).
				Dur("idle_exit", idleExit).
				Msg("starting sync command")

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				log.Error().Err(err).Msg("failed to create app")
				return err
			}
			defer closeApp(a, lk)

			if err := a.EnsureAuthed(); err != nil {
				return err
			}

			mode := appPkg.SyncModeFollow
			if once {
				mode = appPkg.SyncModeOnce
			} else if follow {
				mode = appPkg.SyncModeFollow
			} else {
				mode = appPkg.SyncModeOnce
			}

			log.Debug().Str("mode", string(mode)).Msg("calling app.Sync")
			res, err := a.Sync(ctx, appPkg.SyncOptions{
				Mode:            mode,
				AllowQR:         false,
				DownloadMedia:   downloadMedia,
				RefreshContacts: refreshContacts,
				RefreshGroups:   refreshGroups,
				IdleExit:        idleExit,
			})
			if err != nil {
				log.Error().Err(err).Msg("sync failed")
				return err
			}
			log.Info().Int64("messages_stored", res.MessagesStored).Msg("sync completed")

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"synced":          true,
					"messages_stored": res.MessagesStored,
				})
			}
			fmt.Fprintf(os.Stdout, "Messages stored: %d\n", res.MessagesStored)
			return nil
		},
	}

	cmd.Flags().BoolVar(&once, "once", false, "sync until idle and exit")
	cmd.Flags().BoolVar(&follow, "follow", true, "keep syncing until Ctrl+C")
	cmd.Flags().DurationVar(&idleExit, "idle-exit", 30*time.Second, "exit after being idle (once mode)")
	cmd.Flags().BoolVar(&downloadMedia, "download-media", false, "download media in the background during sync")
	cmd.Flags().BoolVar(&refreshContacts, "refresh-contacts", false, "refresh contacts from session store into local DB")
	cmd.Flags().BoolVar(&refreshGroups, "refresh-groups", false, "refresh joined groups (live) into local DB")
	return cmd
}
