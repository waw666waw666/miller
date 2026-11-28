// Wraps 'sh -c foo bar' or 'cmd /c foo bar', nominally for regression-testing.

//go:build !windows

package platform

func GetShellRunArray(command string) []string {
	return []string{"/bin/sh", "-c", command}
}

// ShellIsCmd reports whether GetShellRunArray runs commands through cmd.exe
// rather than a POSIX sh.
func ShellIsCmd() bool {
	return false
}
