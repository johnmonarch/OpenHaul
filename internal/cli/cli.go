package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/app"
	"github.com/openhaulguard/openhaulguard/internal/apperrors"
	"github.com/openhaulguard/openhaulguard/internal/config"
	"github.com/openhaulguard/openhaulguard/internal/credentials"
	"github.com/openhaulguard/openhaulguard/internal/domain"
	mcpserver "github.com/openhaulguard/openhaulguard/internal/mcp"
	"github.com/openhaulguard/openhaulguard/internal/packet"
	"github.com/openhaulguard/openhaulguard/internal/report"
	"github.com/openhaulguard/openhaulguard/internal/version"
	"github.com/spf13/cobra"
)

type globals struct {
	configPath string
	home       string
	format     string
	offline    bool
	verbose    bool
	debug      bool
	noColor    bool
	yes        bool
}

func Execute() int {
	g := &globals{}
	root := rootCommand(g)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, userError(err))
		return apperrors.ExitCode(err)
	}
	return 0
}

func rootCommand(g *globals) *cobra.Command {
	root := &cobra.Command{
		Use:           "ohg",
		Short:         "OpenHaul Guard carrier verification CLI",
		Version:       version.Version + " (" + version.Commit + ", " + version.Date + ")",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.PersistentFlags().StringVar(&g.configPath, "config", "", "Path to config file")
	root.PersistentFlags().StringVar(&g.home, "home", "", "Override OHG home directory")
	root.PersistentFlags().StringVar(&g.format, "format", "table", "Output format: table, json, markdown")
	root.PersistentFlags().BoolVar(&g.verbose, "verbose", false, "Enable verbose output")
	root.PersistentFlags().BoolVar(&g.debug, "debug", false, "Enable debug logs")
	root.PersistentFlags().BoolVar(&g.offline, "offline", false, "Do not call network sources")
	root.PersistentFlags().BoolVar(&g.noColor, "no-color", false, "Disable terminal colors")
	root.PersistentFlags().BoolVar(&g.yes, "yes", false, "Accept safe defaults in prompts")
	root.AddCommand(setupCommand(g))
	root.AddCommand(initCommand(g))
	root.AddCommand(doctorCommand(g))
	root.AddCommand(carrierCommand(g))
	root.AddCommand(watchCommand(g))
	root.AddCommand(mirrorCommand(g))
	root.AddCommand(configCommand(g))
	root.AddCommand(mcpCommand(g))
	root.AddCommand(packetCommand(g))
	return root
}

func newApp(ctx context.Context, g *globals, migrate bool) (*app.App, error) {
	return app.New(ctx, app.Options{Home: g.home, ConfigPath: g.configPath}, migrate)
}

func setupCommand(g *globals) *cobra.Command {
	var quick bool
	var noBrowser bool
	var webKey string
	var token string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up local OpenHaul Guard config and credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || quick || g.yes {
				return runLocalSetup(cmd, g, "setup")
			}
			return cmd.Help()
		},
	}
	cmd.Flags().BoolVar(&quick, "quick", false, "Fast local setup without government API keys")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print setup URLs instead of opening a browser")
	fmcsaCmd := &cobra.Command{
		Use:   "fmcsa",
		Short: "Configure FMCSA WebKey",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			if err := a.SetupCredential(ctx, "fmcsa", noBrowser, webKey); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "FMCSA WebKey works and was stored.")
			return nil
		},
	}
	fmcsaCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print setup URL instead of opening a browser")
	fmcsaCmd.Flags().StringVar(&webKey, "web-key", "", "FMCSA WebKey")
	_ = fmcsaCmd.Flags().MarkHidden("web-key")
	socrataCmd := &cobra.Command{
		Use:   "socrata",
		Short: "Configure Socrata app token",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			if err := a.SetupCredential(ctx, "socrata", noBrowser, token); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Socrata app token was stored.")
			return nil
		},
	}
	socrataCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print setup URL instead of opening a browser")
	socrataCmd.Flags().StringVar(&token, "token", "", "Socrata app token")
	_ = socrataCmd.Flags().MarkHidden("token")
	cmd.AddCommand(fmcsaCmd, socrataCmd)
	return cmd
}

func initCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Quickly prepare local OpenHaul Guard storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLocalSetup(cmd, g, "init")
		},
	}
}

func runLocalSetup(cmd *cobra.Command, g *globals, commandName string) error {
	ctx := cmd.Context()
	a, err := newApp(ctx, g, true)
	if err != nil {
		return err
	}
	defer a.Close()
	before := a.SetupProgress(ctx)
	if err := a.SetupQuick(ctx); err != nil {
		return err
	}
	if commandName == "setup" {
		a.MarkDefaultSetupComplete(ctx)
	}
	after := a.SetupProgress(ctx)
	resumed := before.ConfigWritten || before.DatabaseInitialized || before.QuickSetupComplete || before.DefaultSetupComplete
	if g.format == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
			"status":     "ok",
			"command":    commandName,
			"home":       a.Config.Home,
			"config":     a.Config.Path,
			"db_path":    a.Config.DBPath,
			"resumed":    resumed,
			"progress":   after,
			"next_step":  "ohg carrier lookup --mc 123456",
			"live_setup": "ohg setup fmcsa",
		})
	}
	out := cmd.OutOrStdout()
	if commandName == "init" {
		fmt.Fprintln(out, "OpenHaul Guard is initialized.")
	} else {
		fmt.Fprintln(out, "Welcome to OpenHaul Guard.")
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "This prepares a private OpenHaul Guard folder on this computer.")
	fmt.Fprintln(out, "No government API key is required for this first step.")
	fmt.Fprintln(out, "")
	if resumed {
		fmt.Fprintln(out, "Found earlier setup progress, so I picked up from there.")
		fmt.Fprintln(out, "")
	}
	fmt.Fprintf(out, "[1/4] Home folder ready: %s\n", a.Config.Home)
	fmt.Fprintf(out, "[2/4] Config file ready: %s\n", a.Config.Path)
	fmt.Fprintf(out, "[3/4] Local database ready: %s\n", a.Config.DBPath)
	fmt.Fprintln(out, "[4/4] Setup state saved: done")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "You can now try a lookup with a sample file:")
	fmt.Fprintln(out, "  ohg carrier lookup --mc 123456 --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "When you want live FMCSA lookups, run:")
	fmt.Fprintln(out, "  ohg setup fmcsa")
	return nil
}

func doctorCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local OpenHaul Guard health",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			result := a.Doctor(ctx)
			if g.format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			if g.format == "markdown" {
				fmt.Fprintln(cmd.OutOrStdout(), "# OpenHaul Guard doctor")
				fmt.Fprintln(cmd.OutOrStdout())
				for _, check := range result.Checks {
					fmt.Fprintf(cmd.OutOrStdout(), "- %s: %s", check.Name, strings.ToUpper(check.Status))
					if check.Message != "" {
						fmt.Fprintf(cmd.OutOrStdout(), " - %s", check.Message)
					}
					if check.Fix != "" {
						fmt.Fprintf(cmd.OutOrStdout(), " (Fix: `%s`)", check.Fix)
					}
					fmt.Fprintln(cmd.OutOrStdout())
				}
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "OpenHaul Guard doctor")
			fmt.Fprintln(cmd.OutOrStdout())
			for _, check := range result.Checks {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s", check.Name, strings.ToUpper(check.Status))
				if check.Message != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " - %s", check.Message)
				}
				if check.Fix != "" && check.Status != "ok" {
					fmt.Fprintf(cmd.OutOrStdout(), "\n  Fix: %s", check.Fix)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nStatus: %s\nSuggested next step:\n  %s\n", result.Status, result.NextStep)
			return nil
		},
	}
}

func carrierCommand(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "carrier", Short: "Carrier lookup, history, and diff commands"}
	cmd.AddCommand(carrierLookupCommand(g))
	cmd.AddCommand(carrierDiffCommand(g))
	return cmd
}

func carrierLookupCommand(g *globals) *cobra.Command {
	var mc, mx, ff, dot, name string
	var force bool
	var maxAge string
	var fixture string
	var saveReport string
	cmd := &cobra.Command{
		Use:   "lookup",
		Short: "Lookup a carrier by MC, MX, FF, USDOT, or name",
		RunE: func(cmd *cobra.Command, args []string) error {
			typ, value, err := exactlyOneIdentifier(map[string]string{"mc": mc, "mx": mx, "ff": ff, "dot": dot, "name": name})
			if err != nil {
				return err
			}
			duration := 24 * time.Hour
			if maxAge != "" {
				duration, err = time.ParseDuration(maxAge)
				if err != nil {
					return apperrors.Wrap(apperrors.CodeInvalidArgs, "invalid --max-age duration", "", err)
				}
			}
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			result, err := a.Lookup(ctx, domain.LookupRequest{
				IdentifierType:  typ,
				IdentifierValue: value,
				ForceRefresh:    force,
				Offline:         g.offline,
				MaxAge:          duration,
				FixturePath:     fixture,
			})
			if err != nil {
				return err
			}
			if saveReport != "" {
				f, err := os.Create(saveReport)
				if err != nil {
					return err
				}
				defer f.Close()
				return report.WriteLookup(f, result, g.format)
			}
			return report.WriteLookup(cmd.OutOrStdout(), result, g.format)
		},
	}
	cmd.Flags().StringVar(&mc, "mc", "", "Motor Carrier docket number")
	cmd.Flags().StringVar(&mx, "mx", "", "Mexico docket number")
	cmd.Flags().StringVar(&ff, "ff", "", "Freight forwarder docket number")
	cmd.Flags().StringVar(&dot, "dot", "", "USDOT number")
	cmd.Flags().StringVar(&name, "name", "", "Carrier name")
	cmd.Flags().BoolVar(&force, "force-refresh", false, "Bypass freshness cache")
	cmd.Flags().StringVar(&maxAge, "max-age", "24h", "Use cache if younger than duration")
	cmd.Flags().StringVar(&saveReport, "save-report", "", "Write report to path")
	cmd.Flags().StringVar(&fixture, "fixture", "", "Hidden test fixture path")
	_ = cmd.Flags().MarkHidden("fixture")
	return cmd
}

func carrierDiffCommand(g *globals) *cobra.Command {
	var mc, dot, since string
	var strict bool
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare local carrier observations",
		RunE: func(cmd *cobra.Command, args []string) error {
			typ, value, err := exactlyOneIdentifier(map[string]string{"mc": mc, "dot": dot})
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			result, err := a.Diff(ctx, typ, value, since, strict)
			if err != nil {
				return err
			}
			return report.WriteDiff(cmd.OutOrStdout(), result, g.format)
		},
	}
	cmd.Flags().StringVar(&mc, "mc", "", "Motor Carrier docket number")
	cmd.Flags().StringVar(&dot, "dot", "", "USDOT number")
	cmd.Flags().StringVar(&since, "since", "90d", "Duration or YYYY-MM-DD start date")
	cmd.Flags().BoolVar(&strict, "strict", false, "Include formatting-only diffs")
	return cmd
}

func watchCommand(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "watch", Short: "Manage and sync watched carriers"}
	cmd.AddCommand(watchAddCommand(g), watchRemoveCommand(g), watchListCommand(g), watchSyncCommand(g), watchReportCommand(g))
	return cmd
}

func watchAddCommand(g *globals) *cobra.Command {
	var mc, dot, label string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a carrier to the local watchlist",
		RunE: func(cmd *cobra.Command, args []string) error {
			typ, value, err := exactlyOneIdentifier(map[string]string{"mc": mc, "dot": dot})
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			if err := a.WatchAdd(ctx, typ, value, label); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added %s %s to watchlist.\n", strings.ToUpper(typ), value)
			return nil
		},
	}
	cmd.Flags().StringVar(&mc, "mc", "", "Motor Carrier docket number")
	cmd.Flags().StringVar(&dot, "dot", "", "USDOT number")
	cmd.Flags().StringVar(&label, "label", "", "Optional watchlist label")
	return cmd
}

func watchRemoveCommand(g *globals) *cobra.Command {
	var mc, dot string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a carrier from the local watchlist",
		RunE: func(cmd *cobra.Command, args []string) error {
			typ, value, err := exactlyOneIdentifier(map[string]string{"mc": mc, "dot": dot})
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			removed, err := a.WatchRemove(ctx, typ, value)
			if err != nil {
				return err
			}
			if g.format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"removed":          removed,
					"identifier_type":  typ,
					"identifier_value": value,
				})
			}
			if removed {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed %s %s from watchlist.\n", strings.ToUpper(typ), value)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s was not active on the watchlist.\n", strings.ToUpper(typ), value)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&mc, "mc", "", "Motor Carrier docket number")
	cmd.Flags().StringVar(&dot, "dot", "", "USDOT number")
	return cmd
}

func watchListCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List watched carriers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			items, err := a.WatchList(ctx)
			if err != nil {
				return err
			}
			if g.format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(items)
			}
			for _, item := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s %s\t%s\n", item.ID, strings.ToUpper(item.IdentifierType), item.IdentifierValue, item.Label)
			}
			return nil
		},
	}
}

func watchSyncCommand(g *globals) *cobra.Command {
	var fixture string
	var force bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Refresh carriers on the watchlist",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			result, err := a.WatchSync(ctx, fixture, force)
			if err != nil {
				return err
			}
			if g.format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Watch sync complete: %d synced, %d failed.\n", result.Synced, result.Failed)
			for _, warning := range result.Warnings {
				fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s - %s\n", warning.Code, warning.Message)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fixture, "fixture", "", "Hidden test fixture path")
	cmd.Flags().BoolVar(&force, "force-refresh", false, "Force live refresh")
	_ = cmd.Flags().MarkHidden("fixture")
	return cmd
}

func watchReportCommand(g *globals) *cobra.Command {
	var since, label string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report changes for watched carriers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			result, err := a.WatchReport(ctx, since, label)
			if err != nil {
				return err
			}
			return report.WriteWatch(cmd.OutOrStdout(), result, g.format)
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Duration or YYYY-MM-DD start date")
	cmd.Flags().StringVar(&label, "label", "", "Only include watched carriers with matching label text")
	return cmd
}

func mirrorCommand(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "mirror", Short: "Manage local bootstrap mirror data"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show local bootstrap mirror status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			status := a.MirrorStatus()
			if g.format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(status)
			}
			if status.Available {
				fmt.Fprintf(cmd.OutOrStdout(), "Bootstrap mirror: OK\nPath: %s\nCarriers: %d\nGenerated: %s\n", status.Path, status.CarrierCount, status.GeneratedAt)
				if status.SourceTimestamp != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Source timestamp: %s\n", status.SourceTimestamp)
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bootstrap mirror: not imported\nPath: %s\n", status.Path)
			if status.Error != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Reason: %s\n", status.Error)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Import one with:")
			fmt.Fprintln(cmd.OutOrStdout(), "  ohg mirror import <path>")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "import <path>",
		Args:  cobra.ExactArgs(1),
		Short: "Import a local JSON bootstrap mirror",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			status, err := a.MirrorImport(ctx, args[0])
			if err != nil {
				return err
			}
			if g.format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(status)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Imported bootstrap mirror: %d carriers\nPath: %s\n", status.CarrierCount, status.Path)
			return nil
		},
	})
	return cmd
}

func configCommand(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Read and update local configuration"}
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print config path",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.Overrides{Home: g.home, ConfigPath: g.configPath})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), cfg.Path)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Print selected config values",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.Overrides{Home: g.home, ConfigPath: g.configPath})
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
				"home":                   cfg.Home,
				"db_path":                cfg.DBPath,
				"mode":                   cfg.Mode,
				"cache.max_age":          cfg.Cache.MaxAge,
				"reports.default_format": cfg.Reports.DefaultFormat,
				"privacy.telemetry":      cfg.Privacy.Telemetry,
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Args:  cobra.ExactArgs(1),
		Short: "Get a config value",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.Overrides{Home: g.home, ConfigPath: g.configPath})
			if err != nil {
				return err
			}
			v, err := getConfigValue(cfg, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Args:  cobra.ExactArgs(2),
		Short: "Set a config value",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.Overrides{Home: g.home, ConfigPath: g.configPath})
			if err != nil {
				return err
			}
			creds := credentials.Store{Home: cfg.Home}
			if args[0] == "fmcsa.web_key" {
				return creds.Set(credentials.UserFMCSAWebKey, args[1])
			}
			if args[0] == "socrata.app_token" {
				return creds.Set(credentials.UserSocrataAppToken, args[1])
			}
			if err := setConfigValue(&cfg, args[0], args[1]); err != nil {
				return err
			}
			return cfg.Save()
		},
	})
	return cmd
}

func mcpCommand(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "mcp", Short: "Run local MCP server"}
	cmd.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Serve MCP over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			return mcpserver.NewServer(
				a,
				cmd.InOrStdin(),
				cmd.OutOrStdout(),
				mcpserver.WithDefaultOffline(g.offline),
			).Run(ctx)
		},
	})
	return cmd
}

func packetCommand(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "packet", Short: "Carrier packet tools"}
	var mc, dot string
	var fixture string
	extractCmd := &cobra.Command{
		Use:   "extract <path>",
		Args:  cobra.ExactArgs(1),
		Short: "Extract structured fields from a carrier packet",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := packet.ExtractReport(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return packet.WriteExtract(cmd.OutOrStdout(), result, g.format)
		},
	}
	checkCmd := &cobra.Command{
		Use:   "check <path>",
		Args:  cobra.ExactArgs(1),
		Short: "Check carrier packet against public records",
		RunE: func(cmd *cobra.Command, args []string) error {
			typ, value, err := exactlyOneIdentifier(map[string]string{"mc": mc, "dot": dot})
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			a, err := newApp(ctx, g, true)
			if err != nil {
				return err
			}
			defer a.Close()
			lookup, err := a.Lookup(ctx, domain.LookupRequest{
				IdentifierType:  typ,
				IdentifierValue: value,
				Offline:         g.offline,
				FixturePath:     fixture,
			})
			if err != nil {
				return err
			}
			result, err := packet.Check(ctx, args[0], lookup)
			if err != nil {
				return err
			}
			return packet.Write(cmd.OutOrStdout(), result, g.format)
		},
	}
	checkCmd.Flags().StringVar(&mc, "mc", "", "Motor Carrier docket number")
	checkCmd.Flags().StringVar(&dot, "dot", "", "USDOT number")
	checkCmd.Flags().StringVar(&fixture, "fixture", "", "Hidden test fixture path")
	_ = checkCmd.Flags().MarkHidden("fixture")
	cmd.AddCommand(extractCmd, checkCmd)
	return cmd
}

func exactlyOneIdentifier(values map[string]string) (string, string, error) {
	var typ, value string
	for k, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if value != "" {
			return "", "", apperrors.New(apperrors.CodeInvalidArgs, "exactly one identifier flag is required", "")
		}
		typ = k
		value = v
	}
	if value == "" {
		return "", "", apperrors.New(apperrors.CodeInvalidArgs, "exactly one identifier flag is required", "")
	}
	return typ, value, nil
}

func getConfigValue(cfg config.Config, key string) (string, error) {
	switch key {
	case "mode":
		return cfg.Mode, nil
	case "cache.max_age":
		return cfg.Cache.MaxAge, nil
	case "reports.default_format":
		return cfg.Reports.DefaultFormat, nil
	case "mcp.transport":
		return cfg.MCP.Transport, nil
	case "mcp.host":
		return cfg.MCP.Host, nil
	case "privacy.telemetry":
		return fmt.Sprint(cfg.Privacy.Telemetry), nil
	case "sources.mirror.enabled":
		return fmt.Sprint(cfg.Sources.Mirror.Enabled), nil
	case "sources.mirror.local_path":
		return cfg.Sources.Mirror.LocalPath, nil
	case "sources.mirror.url":
		return cfg.Sources.Mirror.URL, nil
	default:
		return "", apperrors.New(apperrors.CodeInvalidArgs, "unknown config key", "")
	}
}

func setConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case "mode":
		cfg.Mode = value
	case "cache.max_age":
		cfg.Cache.MaxAge = value
	case "reports.default_format":
		cfg.Reports.DefaultFormat = value
	case "mcp.transport":
		cfg.MCP.Transport = value
	case "mcp.host":
		cfg.MCP.Host = value
	case "privacy.telemetry":
		cfg.Privacy.Telemetry = value == "true"
	case "sources.mirror.enabled":
		cfg.Sources.Mirror.Enabled = value == "true"
	case "sources.mirror.local_path":
		cfg.Sources.Mirror.LocalPath = value
	case "sources.mirror.url":
		cfg.Sources.Mirror.URL = value
	default:
		return apperrors.New(apperrors.CodeInvalidArgs, "unknown config key", "")
	}
	return nil
}

func userError(err error) string {
	if err == nil {
		return ""
	}
	var ohg *apperrors.OHGError
	if errors.As(err, &ohg) && ohg.UserAction != "" {
		return fmt.Sprintf("%s\nFix: %s", ohg.Error(), ohg.UserAction)
	}
	return err.Error()
}
