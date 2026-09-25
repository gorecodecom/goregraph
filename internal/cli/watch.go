package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gorecodecom/goregraph/internal/config"
	"github.com/gorecodecom/goregraph/internal/scan"
	"github.com/gorecodecom/goregraph/internal/watch"
)

func printWatchHelp(out io.Writer) {
	fmt.Fprint(out, `Usage: goregraph watch <start|stop|status|autostart|run> [path] [options]

Quick start from the project directory:
  goregraph watch start .       Start watching this project in the background
  goregraph watch status .      Check whether it is running and updating
  goregraph dashboard open .   Open the dashboard; reload it after changes

"." means the current directory. From elsewhere, pass the project path instead.
The first interactive start asks about automatic startup at future logins;
press Enter for no. For scripts, choose explicitly with --autostart on|off.

To stop watching now, run "goregraph watch stop .". If Autostart is on,
also run "goregraph watch autostart off ." to prevent future login startup.

Recognized workspace roots are selected automatically. For a workspace that
is not recognized, use "goregraph watch start <workspace-path> --workspace".
Stop a running watcher before changing its project/workspace mode.
The watcher updates the agent index and dashboard after file changes. It does
not run tests or application code. Installation never enables or starts it.

All commands:
  start [path] [--workspace] [--autostart on|off]  Start in the background
  stop [path]                                    Stop the running watcher
  status [path]                                  Show live and login status
  autostart on|off [path] [--workspace]          Change login startup
  run [path] [--workspace]                       Run in the foreground
`)
}

func runWatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || isHelp(args[0]) {
		printWatchHelp(stdout)
		return 0
	}
	action := args[0]
	if action != "start" && action != "stop" && action != "status" && action != "autostart" && action != "run" {
		fmt.Fprintf(stderr, "error: unknown watch command %q\n", action)
		return 2
	}
	path := "."
	workspace := false
	workspaceFlag := false
	autoChoice := ""
	positionals := []string{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h", "help":
			printWatchHelp(stdout)
			return 0
		case "--workspace":
			workspace = true
			workspaceFlag = true
		case "--autostart":
			i++
			if i >= len(args) || (args[i] != "on" && args[i] != "off") {
				fmt.Fprintln(stderr, "error: --autostart requires on or off")
				return 2
			}
			autoChoice = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(stderr, "error: unknown watch option %q\n", args[i])
				return 2
			}
			positionals = append(positionals, args[i])
		}
	}
	if action == "autostart" {
		if len(positionals) == 0 || (positionals[0] != "on" && positionals[0] != "off") {
			fmt.Fprintln(stderr, "error: usage: goregraph watch autostart on|off [path]")
			return 2
		}
		autoChoice = positionals[0]
		positionals = positionals[1:]
	} else if action != "start" && autoChoice != "" {
		fmt.Fprintln(stderr, "error: --autostart is supported only by watch start")
		return 2
	}
	if len(positionals) > 1 {
		fmt.Fprintln(stderr, "error: watch accepts only one root path")
		return 2
	}
	if len(positionals) == 1 {
		path = positionals[0]
	}
	root, err := watch.Resolve(path, workspace)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if action == "start" && !workspaceFlag {
		preferred, err := watch.PreferredWorkspace(root)
		if err != nil {
			fmt.Fprintf(stderr, "error: detecting watcher mode: %v\n", err)
			return 1
		}
		if preferred {
			workspace, workspaceFlag, root.Workspace = true, true, true
		}
	}
	if action == "start" || action == "run" || action == "autostart" {
		registered, err := watch.HasSetting(root)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if registered {
			status, err := watch.GetStatus(root)
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			if !workspaceFlag {
				workspace = status.Workspace
				root.Workspace = workspace
			} else if status.Workspace != workspace {
				if action != "start" {
					fmt.Fprintln(stderr, "error: this root is already configured with a different project/workspace mode")
					return 2
				}
				if status.Running {
					fmt.Fprintf(stderr, "error: watcher is running in %s mode; run goregraph watch stop %s, then start it again with --workspace\n", watchMode(status.Workspace), root.Path)
					return 2
				}
				executable, err := os.Executable()
				if err == nil {
					err = watch.ChangeMode(root, executable)
				}
				if err != nil {
					fmt.Fprintf(stderr, "error: changing watcher mode: %v\n", err)
					return 1
				}
			}
		}
	}
	switch action {
	case "status":
		status, err := watch.GetStatus(root)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Root: %s\nMode: %s\nRunning: %t\nAutostart: %t\n", root.Path, watchMode(status.Workspace), status.Running, status.Autostart)
		if output, err := watchOutputPath(root.Path, status.Workspace); err == nil {
			fmt.Fprintf(stdout, "Target output: %s\n", output)
			printWatchOutputStatus(stdout, output)
		} else {
			fmt.Fprintf(stdout, "Output error: %v\n", err)
		}
		if preferred, err := watch.PreferredWorkspace(root); err == nil && preferred && !status.Workspace {
			fmt.Fprintf(stdout, "Warning: this root is a workspace; this project watcher does not update its dashboard at %s. Stop it and start again with --workspace.\n", filepath.Join(root.Path, ".goregraph-workspace"))
		}
		if status.Method != "" {
			fmt.Fprintf(stdout, "Start mechanism: %s\n", status.Method)
		}
		if status.AutostartError != "" {
			fmt.Fprintf(stdout, "Autostart error: %s\n", status.AutostartError)
		}
		if !status.LastSuccess.IsZero() {
			fmt.Fprintf(stdout, "Last successful watcher check: %s\n", status.LastSuccess.Format("2006-01-02 15:04:05 MST"))
		}
		if status.LastError != "" {
			fmt.Fprintf(stdout, "Last error: %s\n", status.LastError)
		}
		return 0
	case "stop":
		stopped, err := watch.RequestStop(root)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if !stopped {
			fmt.Fprintln(stdout, "Watcher is not running.")
			return 0
		}
		fmt.Fprintln(stdout, "Stop requested. The current update may finish first.")
		return 0
	case "autostart":
		executable, err := os.Executable()
		if err == nil {
			err = watch.SetAutostart(root, autoChoice == "on", executable)
		}
		if err != nil {
			fmt.Fprintf(stderr, "error: autostart: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Autostart: %s\n", autoChoice)
		return 0
	case "start":
		registered, err := watch.HasSetting(root)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if !registered && autoChoice == "" {
			autoChoice = "off"
			if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
				fmt.Fprint(stdout, "Start this watcher automatically at future logins? [y/N] ")
				reply, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.EqualFold(strings.TrimSpace(reply), "y") || strings.EqualFold(strings.TrimSpace(reply), "yes") {
					autoChoice = "on"
				}
			}
		}
		if err := watch.Start(root); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if autoChoice != "" {
			executable, err := os.Executable()
			if err == nil {
				err = watch.SetAutostart(root, autoChoice == "on", executable)
			}
			if err != nil {
				fmt.Fprintf(stderr, "error: watcher started, but autostart setup failed: %v\n", err)
				return 1
			}
		}
		fmt.Fprintf(stdout, "Watcher started for %s (%s mode).\n", root.Path, watchMode(workspace))
		return 0
	case "run":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		update := func() error {
			execution := buildExecution{ctx: ctx, options: scan.DefaultBuildOptions()}
			if !workspace {
				needed, err := scan.ProjectUpdateNeeded(ctx, root.Path, scan.BuildTargetAll, execution.options)
				if err != nil {
					return err
				}
				if !needed {
					return nil
				}
			}
			var diagnostic bytes.Buffer
			var code int
			if workspace {
				code = runWorkspaceUpdate([]string{root.Path, "--workspace", root.Path, "--no-update-gitignore"}, io.Discard, &diagnostic, execution)
			} else {
				code = runScan([]string{root.Path, "--target", "all", "--no-update-gitignore", "--no-workspace"}, io.Discard, &diagnostic, true, true, execution)
			}
			if code != 0 {
				message := strings.TrimSpace(diagnostic.String())
				if len(message) > 4096 {
					message = message[:4096]
				}
				return fmt.Errorf("update failed with exit code %d: %s", code, message)
			}
			return nil
		}
		if err := watch.Run(ctx, root, update); err != nil {
			fmt.Fprintf(stderr, "error: watcher: %v\n", err)
			return 1
		}
		return 0
	}
	return 2
}

func watchMode(workspace bool) string {
	if workspace {
		return "workspace"
	}
	return "project"
}

func watchOutputPath(path string, workspace bool) (string, error) {
	if workspace {
		return filepath.Join(path, ".goregraph-workspace"), nil
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", err
	}
	return filepath.Join(path, cfg.OutputDir), nil
}

func printWatchOutputStatus(out io.Writer, output string) {
	body, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if os.IsNotExist(err) {
		fmt.Fprintln(out, "Output: not built yet")
		return
	}
	if err != nil {
		fmt.Fprintf(out, "Output error: %v\n", err)
		return
	}
	var manifest scan.OutputManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		fmt.Fprintf(out, "Output error: invalid manifest: %v\n", err)
		return
	}
	for _, projection := range []struct {
		name   string
		status scan.ProjectionStatus
	}{
		{"Agent generated", manifest.Agent},
		{"Dashboard generated", manifest.Dashboard},
	} {
		if !projection.status.Complete || projection.status.GeneratedAt == "" {
			fmt.Fprintf(out, "%s: not built yet\n", projection.name)
			continue
		}
		generated, err := time.Parse(time.RFC3339, projection.status.GeneratedAt)
		if err != nil {
			fmt.Fprintf(out, "%s: %s\n", projection.name, projection.status.GeneratedAt)
			continue
		}
		fmt.Fprintf(out, "%s: %s\n", projection.name, generated.Local().Format("2006-01-02 15:04:05 MST"))
	}
}
