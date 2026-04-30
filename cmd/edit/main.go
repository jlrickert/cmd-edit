package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Version is overridden at build time via -ldflags by goreleaser.
var Version = "dev"

var completionShell string

var rootCmd = &cobra.Command{
	Use:   "ed [args...]",
	Short: "Editor wrapper (ported from bash) with shell completion support",
	Long: `ed sets up a small editing environment (environment variables like
EDITOR_PROJECT_ROOT, EDITOR_EDIT_TYPE, EDITOR_PATH, EDITOR_FILE_TYPE, EDITOR_PROJECT_TYPE)
and then invokes the configured editor. This binary is intended to be a drop-in
replacement for the original bash script but implemented in Go so Cobra's
completion support can be used.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEd(args)
	},
}

func init() {
	// Wiring rootCmd.Version BEFORE InitDefaultCompletionCmd so cobra picks up
	// the version string when generating the completion subcommand and the
	// auto-injected --version flag.
	rootCmd.Version = Version

	// Initialize the default "completion" command provided by cobra.
	rootCmd.InitDefaultCompletionCmd()

	// Add a --completion <SHELL> persistent flag that writes completions to stdout and exits.
	rootCmd.PersistentFlags().StringVar(&completionShell, "completion", "", "generate shell completion for [bash|zsh|fish|powershell] (prints to stdout and exits)")

	// Register shell completions for the --completion flag.
	// Suggest common shells and avoid file completions.
	rootCmd.RegisterFlagCompletionFunc("completion", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		options := []string{"bash", "zsh", "fish", "powershell", "pwsh"}
		var matches []string
		for _, o := range options {
			if strings.HasPrefix(o, toComplete) {
				matches = append(matches, o)
			}
		}
		// Do not trigger file completion
		return matches, cobra.ShellCompDirectiveNoFileComp
	})

	// If --completion is provided, generate the requested completion and exit before running the command.
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if completionShell == "" {
			return nil
		}
		switch completionShell {
		case "bash":
			if err := rootCmd.GenBashCompletion(os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "error generating bash completion:", err)
				os.Exit(1)
			}
		case "zsh":
			if err := rootCmd.GenZshCompletion(os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "error generating zsh completion:", err)
				os.Exit(1)
			}
		case "fish":
			// include descriptions for fish completions
			if err := rootCmd.GenFishCompletion(os.Stdout, true); err != nil {
				fmt.Fprintln(os.Stderr, "error generating fish completion:", err)
				os.Exit(1)
			}
		case "powershell", "pwsh":
			if err := rootCmd.GenPowerShellCompletion(os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "error generating powershell completion:", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintln(os.Stderr, "unsupported shell for --completion:", completionShell)
			os.Exit(1)
		}
		// Successfully wrote completions; exit now.
		os.Exit(0)
		return nil
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runEd(args []string) error {
	var firstArg string
	if len(args) > 0 {
		firstArg = args[0]
	}

	// Determine EDITOR_PROJECT_ROOT (matches original behavior: dirname of arg,
	// then attempt git rev-parse --show-toplevel from that directory; fallback to dirname)
	projectRoot := "."
	if firstArg != "" {
		dir := firstArg
		if fi, err := os.Stat(firstArg); err == nil && !fi.IsDir() {
			dir = filepath.Dir(firstArg)
		} else if err != nil {
			// if Stat fails we still take dirname(firstArg)
			dir = filepath.Dir(firstArg)
		}

		// Try git rev-parse in that directory
		gitCmd := exec.Command("git", "rev-parse", "--show-toplevel")
		gitCmd.Dir = dir
		if out, err := gitCmd.Output(); err == nil {
			projectRoot = strings.TrimSpace(string(out))
		} else {
			projectRoot = dir
		}
	} else {
		// dirname of empty is "."
		projectRoot = "."
	}

	// Determine EDITOR_EDIT_TYPE and path/file-type behavior
	editType := "unknown"
	var editorPath string
	var editorFileType string

	if firstArg != "" {
		editorPath = firstArg
		if fi, err := os.Stat(firstArg); err == nil {
			if fi.IsDir() {
				editType = "directory"
				// no file type for directories
			} else {
				editType = "file"
				// determine mime type using `file --mime-type -b`
				mimeCmd := exec.Command("file", "--mime-type", "-b", firstArg)
				if out, err := mimeCmd.Output(); err == nil {
					editorFileType = strings.TrimSpace(string(out))
				}
			}
		} else {
			// file doesn't exist, emulate original script which sets "directory" if arg non-empty
			editType = "directory"
			// leave editorFileType empty/unset
			editorFileType = ""
		}
	} else {
		// no arg, leave editorPath empty and file type unset
		editorPath = ""
		editorFileType = ""
		editType = "unknown"
	}

	// Determine project type by presence of specific files in projectRoot
	projectType := "unknown"
	check := func(name string) bool {
		_, err := os.Stat(filepath.Join(projectRoot, name))
		return err == nil
	}
	switch {
	case check("composer.json"):
		projectType = "php"
	case check("version.php"):
		projectType = "php"
	case check("deno.json"):
		projectType = "deno"
	case check("package.json"):
		projectType = "node"
	case check("init.lua"):
		projectType = "lua"
	default:
		projectType = "unknown"
	}

	// Build environment variables to pass to the editor subprocess.
	// We don't modify the parent environment (that's impossible), but we ensure
	// the editor child process sees the same exported variables the bash script set.
	env := os.Environ()
	setEnv := func(k, v string) {
		// Remove any existing variable with key k
		prefix := k + "="
		newEnv := make([]string, 0, len(env))
		found := false
		for _, e := range env {
			if strings.HasPrefix(e, prefix) {
				if !found {
					newEnv = append(newEnv, prefix+v)
					found = true
				}
			} else {
				newEnv = append(newEnv, e)
			}
		}
		if !found {
			newEnv = append(newEnv, prefix+v)
		}
		env = newEnv
	}

	setEnv("EDITOR_PROJECT_ROOT", projectRoot)
	setEnv("EDITOR_EDIT_TYPE", editType)
	if editorPath != "" {
		setEnv("EDITOR_PATH", editorPath)
	}
	if editorFileType != "" {
		setEnv("EDITOR_FILE_TYPE", editorFileType)
	}
	setEnv("EDITOR_PROJECT_TYPE", projectType)

	// Determine which editor to run: prefer VISUAL then EDITOR then fallback to "nano"
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "nano"
	}

	// Prefer running via a shell to support complex editor strings like "code --wait".
	shell := os.Getenv("COMSPEC")
	shellFlag := "/C"
	if shell == "" {
		shell = "/bin/sh"
		shellFlag = "-c"
	}

	// Build the command line for the shell. We must preserve multiple args and escape them.
	cmdLine := editor
	if len(args) > 0 {
		escapedArgs := make([]string, 0, len(args))
		for _, a := range args {
			escapedArgs = append(escapedArgs, shellEscape(a))
		}
		cmdLine += " " + strings.Join(escapedArgs, " ")
	}

	// Execute the editor via the chosen shell, passing the constructed environment.
	cmd := exec.Command(shell, shellFlag, cmdLine)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run and return the error status (if any).
	if err := cmd.Run(); err != nil {
		// propagate error; cobra will handle exit code
		return err
	}
	return nil
}

// shellEscape returns a safely shell-escaped representation of s for POSIX shells.
// It wraps the string in single quotes; single quote characters inside the string
// are represented as '\” which is the standard way to include a single quote inside single-quoted strings.
func shellEscape(s string) string {
	// Fast path
	if s == "" {
		return "''"
	}
	// If s contains no special characters, return as-is to keep commands readable.
	// We still need to ensure spaces are quoted; detect safe characters (alphanum and a small set).
	safe := true
	for _, r := range s {
		if !(r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	// Replace each single quote ' with '\'' sequence
	var buf bytes.Buffer
	buf.WriteByte('\'')
	for _, ch := range s {
		if ch == '\'' {
			buf.WriteString("'\\''")
		} else {
			buf.WriteRune(ch)
		}
	}
	buf.WriteByte('\'')
	return buf.String()
}
