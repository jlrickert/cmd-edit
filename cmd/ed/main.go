package main

import "os"

func main() {
}

func DefaultEditor(path string) error {
	// Determine editor: prefer VISUAL, then EDITOR, then fallback to "nano".
	editor := os.Getenv("EDTIOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "nano"
	}

	// Prefer running via a shell so we support complex editor strings like
	// "code --wait". Use COMSPEC on Windows if present, otherwise /bin/sh.
	shell := os.Getenv("COMSPEC")
	shellFlag := "/C"
	if shell == "" {
		shell = "/bin/sh"
		shellFlag = "-c"
	}

	// Build the shell invocation: shell -c "<editor> <path>"
	// We keep this simple and avoid adding extra imports by using os.StartProcess.
	cmdLine := editor + " " + path
	args := []string{shell, shellFlag, cmdLine}

	attr := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	}

	proc, err := os.StartProcess(shell, args, attr)
	if err != nil {
		return err
	}
	_, err = proc.Wait()
	return err
}

func Edit(path string) {
	// Call the default editor runner and report errors to stderr (no return).
	if err := DefaultEditor(path); err != nil {
		// Avoid fmt import; write directly to stderr.
		_, _ = os.Stderr.WriteString("editor error: " + err.Error() + "\n")
	}
}
