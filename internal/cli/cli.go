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
	root.AddCommand(doctorCommand(g))
	root.AddCommand(carrierCommand(g))
	root.AddCommand(watchCommand(g))
	root.AddCommand(configCommand(g))
	root.AddCommand(mcpCommand(g))
	root.AddCommand(packetCommand())
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
			ctx := cmd.Context()
			a, err := newApp(ctx, g, false)
			if err != nil {
				return err
			}
			defer a.Close()
			if len(args) == 0 || quick || g.yes {
				if err := a.SetupQuick(ctx); err != nil {
					return err
				}
				if g.format == "json" {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"status":    "ok",
						"home":      a.Config.Home,
						"db_path":   a.Config.DBPath,
						"next_step": "ohg carrier lookup --mc 123456",
					})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Welcome to OpenHaul Guard.")
				fmt.Fprintln(cmd.OutOrStdout(), "")
				fmt.Fprintln(cmd.OutOrStdout(), "[1/4] Creating local directories... done")
				fmt.Fprintln(cmd.OutOrStdout(), "[2/4] Creating config file... done")
				fmt.Fprintln(cmd.OutOrStdout(), "[3/4] Creating local database... done")
				fmt.Fprintln(cmd.OutOrStdout(), "[4/4] Running quick setup... done")
				fmt.Fprintln(cmd.OutOrStdout(), "")
				fmt.Fprintln(cmd.OutOrStdout(), "You can now try:")
				fmt.Fprintln(cmd.OutOrStdout(), "  ohg carrier lookup --mc 123456 --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json")
				fmt.Fprintln(cmd.OutOrStdout(), "")
				fmt.Fprintln(cmd.OutOrStdout(), "For fresher live lookups later, run:")
				fmt.Fprintln(cmd.OutOrStdout(), "  ohg setup fmcsa")
				return nil
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
	cmd.AddCommand(watchAddCommand(g), watchListCommand(g), watchSyncCommand(g))
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
			manifest := map[string]any{
				"status":    "developer_preview",
				"transport": "stdio",
				"tools":     []string{"carrier_lookup", "carrier_diff"},
				"note":      "Full MCP JSON-RPC serving is planned after CLI JSON schemas stabilize.",
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(manifest)
		},
	})
	return cmd
}

func packetCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "packet", Short: "Carrier packet tools"}
	cmd.AddCommand(&cobra.Command{
		Use:   "check <path>",
		Args:  cobra.ExactArgs(1),
		Short: "Check carrier packet against public records",
		RunE: func(cmd *cobra.Command, args []string) error {
			return apperrors.New(apperrors.CodePacketParseFailed, "packet checker is not implemented in this first build", "Use carrier lookup and diff first; packet check is the next MVP extension")
		},
	})
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
