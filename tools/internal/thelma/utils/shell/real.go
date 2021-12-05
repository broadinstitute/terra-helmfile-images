package shell

import (
	"bytes"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"strings"
)

const maxErrorBufLenBytes = 5 * 1024 // 5 kb
const maxCmdId = 1<<24 - 1
const cmdIdFormat = "%06x"
const eol = '\n'
const defaultLogLevel = zerolog.InfoLevel

// Options for a shell runner
type Options struct {
	// Logger this Runner should use
	Logger *zerolog.Logger
}

// RealRunner is an implementation of the Runner interface that actually executes shell commands
type RealRunner struct {
	logger zerolog.Logger
}

// NewDefaultRunner constructs a new RealRunner
func NewDefaultRunner() Runner {
	return NewRunner(Options{})
}

// NewRunner constructs a new Runner with the given options
func NewRunner(options Options) Runner {
	logger := log.Logger
	if options.Logger != nil {
		logger = *options.Logger
	}

	return &RealRunner{
		logger: logger,
	}
}

// Run runs a Command, returning an error if the command exits non-zero
func (r *RealRunner) Run(cmd Command) error {
	return r.RunWith(cmd, RunOptions{})
}

// Capture runs a Command, streaming stdout and stderr to the given writers.
// An error is returned if the command exits non-zero.
func (r *RealRunner) RunWith(cmd Command, opts RunOptions) error {
	// Handle options into local variables
	logger := r.logger
	if opts.Logger != nil {
		logger = *opts.Logger
	}
	level := defaultLogLevel
	if opts.LogLevel != nil {
		level = *opts.LogLevel
	}
	stderr := opts.Stderr
	stdout := opts.Stdout

	// Generate an id to uniquely identify this command in log messages and add to Log context
	id := r.generateCommandId()
	logger = logger.With().Str("cmd", id).Logger()

	// Wrap user-supplied stderr writer in a new io.Writer that records stderr output
	errCapture := newCapturingWriter(maxErrorBufLenBytes, logger, stderr)

	// Wrap user-supplied stdout and stderr in new io.Writers that log messages at debug level
	stdout = newLoggingWriter(zerolog.DebugLevel, logger.With().Str("stream", "stdout").Logger(), "[out] ", stdout)
	stderr = newLoggingWriter(zerolog.DebugLevel, logger.With().Str("stream", "stderr").Logger(), "[err] ", errCapture)

	// Convert our command arguments to exec.Cmd struct
	execCmd := exec.Command(cmd.Prog, cmd.Args...)
	execCmd.Dir = cmd.Dir
	if !cmd.PristineEnv {
		execCmd.Env = os.Environ()
	}
	execCmd.Env = append(execCmd.Env, cmd.Env...)
	execCmd.Stdout = stdout
	execCmd.Stderr = stderr

	logger.WithLevel(level).Str("dir", cmd.Dir).Msgf("Executing: %q", cmd.PrettyFormat())

	err := execCmd.Run()
	if err != nil {
		logger.Debug().Msgf("Command failed: %v\n", err)

		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExitError{
				Command:  cmd,
				ExitCode: exitErr.ExitCode(),
				Stderr:   errCapture.String(),
			}
		} else {
			return &Error{
				Command: cmd,
				err:     err,
			}
		}
	}

	return nil
}

// Generates an id to uniquely identify commands in log messages
func (r *RealRunner) generateCommandId() string {
	return fmt.Sprintf(cmdIdFormat, rand.Intn(maxCmdId))
}

// An io.Writer that captures data it receives with Write() into a buffer and optionally forwards to another writer
type capturingWriter struct {
	len    int
	maxLen int
	buf    *bytes.Buffer
	logger zerolog.Logger
	inner  io.Writer
}

func newCapturingWriter(rolloverLen int, logger zerolog.Logger, inner io.Writer) *capturingWriter {
	return &capturingWriter{
		maxLen: rolloverLen,
		buf:    bytes.NewBuffer([]byte{}),
		inner:  inner,
		logger: logger,
	}
}

func (cw *capturingWriter) String() string {
	return cw.buf.String()
}

func (cw *capturingWriter) Write(p []byte) (n int, err error) {
	if len(p) > cw.maxLen {
		if cw.len > 0 {
			cw.rollover()
		}

		cw.logger.Warn().
			Int("max-len", cw.maxLen).
			Str("content", string(p)).
			Msgf("capturing writer: message too long (%d bytes), won't capture", len(p))

		n, err = len(p), nil
	} else {
		if cw.len+len(p) > cw.maxLen {
			cw.rollover()
		}

		n, err = cw.buf.Write(p)
		if err != nil {
			return n, fmt.Errorf("capturing writer: error writing to buffer: %v", err)
		}
		cw.len += n
	}

	if cw.inner == nil {
		return n, err
	}

	return cw.inner.Write(p)
}

func (cw *capturingWriter) rollover() {
	cw.logger.Warn().
		Int("current-len", cw.len).
		Int("max-len", cw.maxLen).
		Str("content", cw.buf.String()).
		Msg("capturing writer: buffer rolled over")
	cw.buf = bytes.NewBuffer([]byte{})
	cw.len = 0
}

// An io.Writer that logs messages that are sent to it with Write() and optionally forwards to another io.Writer
type loggingWriter struct {
	level  zerolog.Level
	logger zerolog.Logger
	prefix string
	inner  io.Writer
}

func newLoggingWriter(level zerolog.Level, logger zerolog.Logger, prefix string, inner io.Writer) *loggingWriter {
	return &loggingWriter{
		level:  level,
		logger: logger,
		prefix: prefix,
		inner:  inner,
	}
}

func (lw *loggingWriter) Write(p []byte) (n int, err error) {
	n, err = lw.streamLinesToLog(p)

	if lw.inner == nil {
		return n, err
	}

	return lw.inner.Write(p)
}

func (lw *loggingWriter) streamLinesToLog(p []byte) (n int, err error) {
	p2 := make([]byte, len(p))
	copy(p2, p)

	eolStr := string(eol)

	buf := bytes.NewBuffer(p2)
	for {
		line, err := buf.ReadString(eol)
		n += len(line)

		if err == nil || len(line) > 0 {
			line = strings.TrimSuffix(line, eolStr)
			lw.logger.WithLevel(lw.level).Msgf("%s%s", lw.prefix, line)
		}

		if err == io.EOF {
			return n, nil
		} else if err != nil {
			return n, fmt.Errorf("logging writer: error reading from buffer: %v", err)
		}
	}
}
