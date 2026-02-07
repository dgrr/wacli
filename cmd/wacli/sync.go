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
	"github.com/steipete/wacli/internal/rpc"
	"go.mau.fi/whatsmeow/types"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var once bool
	var follow bool
	var idleExit time.Duration
	var downloadMedia bool
	var refreshContacts bool
	var refreshGroups bool
	var enableRPC bool
	var rpcAddr string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync messages (requires prior auth; never shows QR)",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logging.WithComponent("sync")
			log.Info().
				Bool("once", once).
				Bool("follow", follow).
				Bool("rpc", enableRPC).
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

			// Start RPC server if enabled
			var rpcServer *rpc.Server
			if enableRPC {
				rpcServer, err = rpc.New(rpc.Options{
					Addr: rpcAddr,
					DB:   a.DB(),
				})
				if err != nil {
					return fmt.Errorf("create rpc server: %w", err)
				}
				if err := rpcServer.Start(); err != nil {
					return fmt.Errorf("start rpc server: %w", err)
				}
				defer func() {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_ = rpcServer.Stop(shutdownCtx)
				}()
				if rpcServer.IsUnixSocket() {
					fmt.Fprintf(os.Stderr, "RPC server listening on %s\n", rpcAddr)
				} else {
					fmt.Fprintf(os.Stderr, "RPC server listening on http://%s\n", rpcAddr)
				}
			}

			// After connect callback to set WA client for RPC
			var afterConnect func(context.Context) error
			if enableRPC && rpcServer != nil {
				afterConnect = func(ctx context.Context) error {
					if wa := a.WA(); wa != nil {
						rpcServer.SetWA(&syncWAWrapper{wa: wa})
					}
					rpcServer.SetSyncRunning(true)
					return nil
				}
			}

			log.Debug().Str("mode", string(mode)).Msg("calling app.Sync")
			res, err := a.Sync(ctx, appPkg.SyncOptions{
				Mode:            mode,
				AllowQR:         false,
				AfterConnect:    afterConnect,
				DownloadMedia:   downloadMedia,
				RefreshContacts: refreshContacts,
				RefreshGroups:   refreshGroups,
				IdleExit:        idleExit,
			})

			if rpcServer != nil {
				rpcServer.SetSyncRunning(false)
			}

			if err != nil {
				log.Error().Err(err).Msg("sync failed")
				return err
			}
			log.Info().Int64("messages_stored", res.MessagesStored).Msg("sync completed")

			if flags.asJSON {
				result := map[string]any{
					"synced":          true,
					"messages_stored": res.MessagesStored,
				}
				if enableRPC {
					result["rpc_addr"] = rpcAddr
				}
				return out.WriteJSON(os.Stdout, result)
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
	cmd.Flags().BoolVar(&enableRPC, "rpc", false, "start HTTP RPC server alongside sync")
	cmd.Flags().StringVar(&rpcAddr, "rpc-addr", "localhost:5555", "RPC server listen address")
	return cmd
}

// syncWAWrapper adapts the app.WAClient to rpc.WAClient interface.
type syncWAWrapper struct {
	wa appPkg.WAClient
}

func (w *syncWAWrapper) IsConnected() bool {
	return w.wa != nil && w.wa.IsConnected()
}

func (w *syncWAWrapper) SendText(ctx context.Context, to types.JID, text string) (types.MessageID, error) {
	return w.wa.SendText(ctx, to, text)
}

func (w *syncWAWrapper) ResolveChatName(ctx context.Context, chat types.JID, pushName string) string {
	return w.wa.ResolveChatName(ctx, chat, pushName)
}
