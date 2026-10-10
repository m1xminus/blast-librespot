//go:build !windows

package output

import (
	"fmt"
	"os"
	"sync"
	"syscall"
)

func newPipeOutput(opts *NewOutputOptions) (out *pipeOutput, err error) {
	out = &pipeOutput{
		reader:         opts.Reader,
		volume:         opts.InitialVolume,
		err:            make(chan error, 2),
		externalVolume: opts.ExternalVolume,
		volumeUpdate:   opts.VolumeUpdate,
	}

	out.cond = sync.NewCond(&out.lock)

	out.transform, err = newPipeTransform(opts.OutputPipeFormat)
	if err != nil {
		return nil, err
	}

	// Open the FIFO for writing. When not waiting for a reader, open it as
	// non-blocking to cause an error if there is no reader. When waiting for a
	// reader (e.g. snapcast with dryout), the blocking open will wait until a
	// reader connects.
	flags := os.O_WRONLY
	if !opts.OutputPipeWaitForReader {
		flags |= syscall.O_NONBLOCK
	}

	// A local *os.File (not out.file, now the generic io.WriteCloser the
	// Windows variant also needs to satisfy) so Fd() below stays available
	// -- that's an *os.File-only method, not part of the interface.
	f, err := os.OpenFile(opts.OutputPipe, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open fifo: %w", err)
	}

	if !opts.OutputPipeWaitForReader {
		// Restore blocking mode now that we are sure we have a reader.
		if err := syscall.SetNonblock(int(f.Fd()), false); err != nil {
			return nil, fmt.Errorf("failed to set blocking mode on fifo: %w", err)
		}
	}

	out.file = f

	go out.outputLoop()

	return out, nil
}
