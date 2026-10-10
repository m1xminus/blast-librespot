//go:build windows

package output

// BLAST patch: upstream's Windows build of this backend was a stub that
// always returned "pipe output is not supported on Windows" -- Windows has
// no POSIX FIFO equivalent os.OpenFile can open, and a real Windows named
// pipe needs the Win32 CreateNamedPipe/ConnectNamedPipe API, which this
// project doesn't otherwise depend on (no CGO win32 bindings, no
// go-winio). Rather than add that, this dials a plain TCP loopback
// connection instead: on Windows, audio_output_pipe in config.yml is a
// "host:port" address (e.g. "127.0.0.1:51820"), not a filesystem path.
// The caller (BLAST itself) is expected to already be listening on that
// port before this process starts -- same "the app launching the
// subprocess is the server" shape as the Unix FIFO case, where the reader
// is expected to already exist. A net.Conn satisfies the same
// io.WriteCloser pipeOutput.file now holds (see driver-pipe.go), so the
// rest of the output path (outputLoop, Pause/Resume/Close) is completely
// unchanged and shared with every other platform.

import (
	"fmt"
	"net"
	"sync"
	"time"
)

const pipeDialTimeout = 10 * time.Second

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

	conn, err := net.DialTimeout("tcp", opts.OutputPipe, pipeDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to audio pipe socket %q: %w", opts.OutputPipe, err)
	}
	out.file = conn

	go out.outputLoop()

	return out, nil
}
