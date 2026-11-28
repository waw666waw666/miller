package lib

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeFileNameForPopenSh(t *testing.T) {
	cases := map[string]string{
		"abc":  `'abc'`,
		"a b":  `'a b'`,
		"a;b":  `'a;b'`,
		"a'b":  `'a'\''b'`,
		`a"b`:  `'a"b'`,
		"a%b%": `'a%b%'`,
		"-n":   `'./-n'`,
		"":     `''`,
	}
	for filename, expected := range cases {
		actual, err := escapeFileNameForPopen(filename, false)
		assert.NoError(t, err, "filename %q", filename)
		assert.Equal(t, expected, actual, "filename %q", filename)
	}
}

func TestEscapeFileNameForPopenCmd(t *testing.T) {
	cases := map[string]string{
		"abc":               `"abc"`,
		`C:\My Files\a.csv`: `"C:\My Files\a.csv"`,
		"file (1).csv":      `"file (1).csv"`,
		"a;b,c=d.csv":       `"a;b,c=d.csv"`,
		"a'b.csv":           `"a'b.csv"`,
		"-n":                `".\-n"`,
	}
	for filename, expected := range cases {
		actual, err := escapeFileNameForPopen(filename, true)
		assert.NoError(t, err, "filename %q", filename)
		assert.Equal(t, expected, actual, "filename %q", filename)
	}

	rejected := []string{
		"x&calc.exe",
		"x|calc.exe",
		"x>y",
		"x<y",
		"x^y",
		"%PATH%",
		"x%y",
		"!PATH!",
		`x"y`,
	}
	for _, filename := range rejected {
		_, err := escapeFileNameForPopen(filename, true)
		assert.Error(t, err, "filename %q", filename)
	}
}

// shellTestFilenames are filenames containing shell metacharacters. If
// quoting regresses, payloads such as id>pwned.txt will run, so tests using
// these run in a temporary directory.
var shellTestFilenames = []string{
	"plain.csv",
	"with space.csv",
	"x;id>pwned.txt",
	"x|id",
	"x&id",
	"x`id`",
	"x$(id)",
	"x${HOME}",
	"x'$(id)'",
	`x"$(id)"`,
	`x\'y`,
	"'",
	"''",
	`"`,
	"a\nb",
	"a\tb",
	"*",
	"~",
	"#x",
	"x>y<z",
}

// TestEscapeFileNameForPopenRoundTrip checks that, for filenames containing
// shell metacharacters, /bin/sh sees exactly the original filename as a
// single word, with nothing expanded or executed.
func TestEscapeFileNameForPopenRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	dir := t.TempDir()
	for _, filename := range shellTestFilenames {
		escaped, err := escapeFileNameForPopen(filename, false)
		assert.NoError(t, err, "filename %q", filename)
		cmd := exec.Command("/bin/sh", "-c", "printf '%s' "+escaped)
		cmd.Dir = dir
		output, err := cmd.Output()
		assert.NoError(t, err, "filename %q", filename)
		assert.Equal(t, filename, string(output), "filename %q", filename)
	}
}

// TestOpenFileForReadPrepipe checks that with --prepipe (not --prepipex) the
// file is given to the command as its stdin, so filenames with shell
// metacharacters are read correctly and nothing in them is executed.
func TestOpenFileForReadPrepipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("filenames include characters not allowed on Windows")
	}
	dir := t.TempDir()
	for _, filename := range shellTestFilenames {
		path := filepath.Join(dir, filename)
		contents := "contents of " + filename + "\n"
		assert.NoError(t, os.WriteFile(path, []byte(contents), 0644), "filename %q", filename)

		handle, err := OpenFileForRead(path, "cat", false, FileInputEncodingDefault)
		assert.NoError(t, err, "filename %q", filename)
		if err != nil {
			continue
		}
		output, err := io.ReadAll(handle)
		assert.NoError(t, err, "filename %q", filename)
		assert.Equal(t, contents, string(output), "filename %q", filename)
		_ = handle.Close()
		assert.NoError(t, os.Remove(path), "filename %q", filename)
	}
	_, err := os.Stat("pwned.txt")
	assert.True(t, os.IsNotExist(err), "pwned.txt should not exist")
}

// TestOpenFileForReadPrepipeComplete checks that all of the prepipe command's
// output is read, even when the command exits before the reader has drained
// the pipe.
func TestOpenFileForReadPrepipeComplete(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses cat")
	}
	path := filepath.Join(t.TempDir(), "big.txt")
	contents := []byte(strings.Repeat("abcdefghijklmnopqrstuvwxyz0123456789\n", 50000))
	assert.NoError(t, os.WriteFile(path, contents, 0644))

	for _, prepipeIsRaw := range []bool{false, true} {
		for i := 0; i < 10; i++ {
			handle, err := OpenFileForRead(path, "cat", prepipeIsRaw, FileInputEncodingDefault)
			assert.NoError(t, err)
			if err != nil {
				continue
			}
			output, err := io.ReadAll(handle)
			assert.NoError(t, err)
			assert.Equal(t, len(contents), len(output), "prepipeIsRaw=%v", prepipeIsRaw)
			_ = handle.Close()
		}
	}
}

func TestOpenFileForReadPrepipeMissingFile(t *testing.T) {
	_, err := OpenFileForRead(
		filepath.Join(t.TempDir(), "nonesuch"), "cat", false, FileInputEncodingDefault,
	)
	assert.Error(t, err)
}
