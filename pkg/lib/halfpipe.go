package lib

import (
	"fmt"
	"os"

	"github.com/johnkerl/miller/v6/pkg/platform"
)

// OpenOutboundHalfPipe returns a handle to a process. Writing to that handle
// writes to the process' stdin. The process' stdout and stderr are the current
// process' stdout and stderr.
//
// This is for pipe-output-redirection in the Miller put/filter DSL.
//
// Note I am not using os.exec.Cmd which is billed as being simpler than using
// os.StartProcess. It may indeed be simpler when you want to handle the
// subprocess' stdin/stdout/stderr all three within the parent process.  Here I
// found it much easier to use os.StartProcess to let the stdout/stderr run
// free.

func OpenOutboundHalfPipe(commandString string) (*os.File, error) {
	if !shellOutEnabled {
		return nil, fmt.Errorf("piped redirects are disabled by --no-shell / MLR_NO_SHELL")
	}
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	var procAttr os.ProcAttr
	procAttr.Files = []*os.File{
		readPipe,
		os.Stdout,
		os.Stderr,
	}

	// /bin/sh -c "..." or cmd /c "..."
	shellRunArray := platform.GetShellRunArray(commandString)

	process, err := os.StartProcess(shellRunArray[0], shellRunArray, &procAttr)
	if err != nil {
		return nil, err
	}

	go func() { _, _ = process.Wait() }()

	return writePipe, nil
}

// OpenInboundHalfPipe returns a handle to a process. Reading from that handle
// reads from the process' stdout. The process' stderr is the current process'
// stderr. If stdinPath is empty, the process' stdin is the current process'
// stdin; otherwise Miller opens stdinPath and gives it to the process as its
// stdin. That way the filename is never part of the shell command line, so
// it can't be used for shell injection, on any platform.
//
// This is for the Miller prepipe feature.
//
// Note I am not using os.exec.Cmd which is billed as being simpler than using
// os.StartProcess. It may indeed be simpler when you want to handle the
// subprocess' stdin/stdout/stderr all three within the parent process.  Here I
// found it much easier to use os.StartProcess to let the stdin/stderr run
// free.

func OpenInboundHalfPipe(commandString string, stdinPath string) (*os.File, error) {
	if !shellOutEnabled {
		return nil, fmt.Errorf("--prepipe/--prepipex are disabled by --no-shell / MLR_NO_SHELL")
	}

	stdin := os.Stdin
	if stdinPath != "" {
		handle, err := os.Open(stdinPath)
		if err != nil {
			return nil, err
		}
		// The child process gets its own copy of the descriptor at start, so
		// ours can be closed once StartProcess has returned.
		defer func() { _ = handle.Close() }()
		stdin = handle
	}

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	var procAttr os.ProcAttr
	procAttr.Files = []*os.File{
		stdin,
		writePipe,
		os.Stderr,
	}

	// /bin/sh -c "..." or cmd /c "..."
	shellRunArray := platform.GetShellRunArray(commandString)

	process, err := os.StartProcess(shellRunArray[0], shellRunArray, &procAttr)
	// The child has its own copy of the write end. Closing ours means that
	// once the child exits, the reader sees EOF after draining everything
	// the child wrote.
	_ = writePipe.Close()
	if err != nil {
		_ = readPipe.Close()
		return nil, err
	}

	// Reap the child. Don't close readPipe here: the caller may not have
	// read everything yet, and closing it would discard the rest of the
	// output. The caller closes readPipe when done with it.
	go func(process *os.Process) {
		_, err := process.Wait()
		if err != nil {
			fmt.Fprintf(os.Stderr, "mlr: %v\n", err)
		}
	}(process)

	return readPipe, nil
}
