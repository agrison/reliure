package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

var moveToTrash = systemMoveToTrash

func systemMoveToTrash(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("osascript", "-e", `tell application "Finder" to delete POSIX file `+quoteAppleScript(path)).Run()
	case "windows":
		script := `
Add-Type -AssemblyName Microsoft.VisualBasic
[Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($args[0], 'OnlyErrorDialogs', 'SendToRecycleBin')
`
		return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script, path).Run()
	case "linux":
		if gio, err := exec.LookPath("gio"); err == nil {
			return exec.Command(gio, "trash", path).Run()
		}
		if kioclient, err := exec.LookPath("kioclient5"); err == nil {
			return exec.Command(kioclient, "move", path, "trash:/").Run()
		}
		if kioclient, err := exec.LookPath("kioclient"); err == nil {
			return exec.Command(kioclient, "move", path, "trash:/").Run()
		}
		return fmt.Errorf("no system trash command found")
	default:
		return fmt.Errorf("moving files to trash is unsupported on %s", runtime.GOOS)
	}
}

func quoteAppleScript(s string) string {
	out := `"`
	for _, r := range s {
		switch r {
		case '\\', '"':
			out += `\` + string(r)
		default:
			out += string(r)
		}
	}
	return out + `"`
}
