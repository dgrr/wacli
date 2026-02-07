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

func newRPCCmd(flags *rootFlags) *cobra.Command {
	var addr string
	var enableSync bool
	var idleExit time.Duration
	var downloadMedia bool
	var refreshContacts bool
	var refreshGroups bool

	cmd := &cobra.Command{
		Use:   "rpc",
		Short: "Start HTTP RPC server (optionally with sync)",
		Long: `Start an HTTP RPC server that allows querying chats, messages,
and sending messages via HTTP requests. Can run with or without
active sync.

Endpoints:
  GET  /status    - Server status
  GET  /chats     - List chats
  GET  /messages  - Get messages (requires chat_jid param)
  POST /search    - Search messages
  POST /send      - Send a message
  GET  /ping      - Health check

Examples:
  # Start RPC server only (queries existing DB)
  wacli rpc

  # Start RPC server with active sync
  wacli rpc --sync

  # Use custom port
  wacli rpc --addr localhost:8080`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logging.WithComponent("rpc")
			log.Info().
				Str("addr", addr).
				Bool("sync", enableSync).
				Msg("starting RPC command")

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			// Create app
			a, lk, err := newApp(ctx, flags, enableSync, !enableSync)
			if err != nil {
				log.Error().Err(err).Msg("failed to create app")
				return err
			}
			defer closeApp(a, lk)

			// Create RPC server
			rpcServer, err := rpc.New(rpc.Options{
				Addr: addr,
				DB:   a.DB(),
			})
			if err != nil {
				return fmt.Errorf("create rpc server: %w", err)
			}

			// Start RPC server
			if err := rpcServer.Start(); err != nil {
				return fmt.Errorf("start rpc server: %w", err)
			}
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = rpcServer.Stop(shutdownCtx)
			}()

			fmt.Fprintf(os.Stderr, "RPC server listening on http://%s\n", addr)

			// If sync is enabled, connect and run sync
			if enableSync {
				if err := a.EnsureAuthed(); err != nil {
					return err
				}

				// Set WA client for RPC server after connection
				afterConnect := func(ctx context.Context) error {
					if wa := a.WA(); wa != nil {
						rpcServer.SetWA(&waWrapper{wa: wa})
					}
					rpcServer.SetSyncRunning(true)
					return nil
				}

				fmt.Fprintln(os.Stderr, "Starting sync with RPC server...")
				res, err := a.Sync(ctx, appPkg.SyncOptions{
					Mode:            appPkg.SyncModeFollow,
					AllowQR:         false,
					AfterConnect:    afterConnect,
					DownloadMedia:   downloadMedia,
					RefreshContacts: refreshContacts,
					RefreshGroups:   refreshGroups,
					IdleExit:        idleExit,
				})
				rpcServer.SetSyncRunning(false)
				if err != nil {
					log.Error().Err(err).Msg("sync failed")
					return err
				}

				if flags.asJSON {
					return out.WriteJSON(os.Stdout, map[string]any{
						"rpc_addr":        addr,
						"synced":          true,
						"messages_stored": res.MessagesStored,
					})
				}
				fmt.Fprintf(os.Stdout, "RPC stopped. Messages stored: %d\n", res.MessagesStored)
			} else {
				// RPC-only mode: just wait for shutdown
				fmt.Fprintln(os.Stderr, "RPC server running (no sync). Press Ctrl+C to stop.")
				<-ctx.Done()
				fmt.Fprintln(os.Stderr, "\nShutting down...")

				if flags.asJSON {
					return out.WriteJSON(os.Stdout, map[string]any{
						"rpc_addr": addr,
						"stopped":  true,
					})
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "localhost:5555", "RPC server listen address")
	cmd.Flags().BoolVar(&enableSync, "sync", false, "run sync alongside RPC server")
	cmd.Flags().DurationVar(&idleExit, "idle-exit", 0, "exit after being idle (0 = never)")
	cmd.Flags().BoolVar(&downloadMedia, "download-media", false, "download media in background during sync")
	cmd.Flags().BoolVar(&refreshContacts, "refresh-contacts", false, "refresh contacts from session store")
	cmd.Flags().BoolVar(&refreshGroups, "refresh-groups", false, "refresh joined groups")

	return cmd
}

// waWrapper adapts the app.WAClient to rpc.WAClient interface.
type waWrapper struct {
	wa appPkg.WAClient
}

func (w *waWrapper) IsConnected() bool {
	return w.wa != nil && w.wa.IsConnected()
}

func (w *waWrapper) SendText(ctx context.Context, to types.JID, text string) (types.MessageID, error) {
	return w.wa.SendText(ctx, to, text)
}

func (w *waWrapper) ResolveChatName(ctx context.Context, chat types.JID, pushName string) string {
	return w.wa.ResolveChatName(ctx, chat, pushName)
}
