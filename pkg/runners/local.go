package runners

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"

	"orch.io/pkg/utils"
)

type LocalRunner struct {
	name string
	env  map[string]string
}

func (t *LocalRunner) Name() string {
	return t.name
}

func (t *LocalRunner) Type() RunnerType {
	return RunnerTypeLocal
}

func (t *LocalRunner) Capabilities() Capabilities {
	return Capabilities{Exec: true, FileCopy: true}
}

func (t *LocalRunner) ValidateAndInitialize() error {
	return nil
}

func (t *LocalRunner) Exec(ctx context.Context, req ExecCommand) (*ExecResult, error) {
	cmd := exec.Command(req.Command[0], req.Command[1:]...)
	if req.Stdin != nil {
		cmd.Stdin = req.Stdin
	}
	var stdout bytes.Buffer
	if req.Stdout != nil {
		cmd.Stdout = io.MultiWriter(req.Stdout, &stdout)
	} else {
		cmd.Stdout = &stdout
	}
	var stderr bytes.Buffer
	if req.Stderr != nil {
		cmd.Stderr = io.MultiWriter(req.Stderr, &stderr)
	} else {
		cmd.Stderr = &stderr
	}

	cmd.Env = append(os.Environ(), utils.MapToEnvSlice(t.env, req.Env)...)
	cmd.Dir = req.WorkingDir

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)
	ensureExecWriterLineEnds(req.Stdout, req.Stderr)

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}

	return &ExecResult{
		ExitCode: exitCode,
		Duration: duration,
		Error:    err,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}, nil
}

func (t *LocalRunner) CopyFile(ctx context.Context, req FileCopyRequest) (*FileCopyResult, error) {
	var srcFS, dstFS utils.FSWithPath
	srcFS = utils.FSWithPath{FS: &utils.LocalFS{}, Path: req.Source}
	dstFS = utils.FSWithPath{FS: &utils.LocalFS{}, Path: req.Destination}

	copyRes, err := utils.FSCopy(srcFS, dstFS, utils.FSCopyOptions{
		Recursive: req.Recursive,
		Overwrite: req.Overwrite,
	})

	var totalBytes int64
	var totalFiles int
	var duration time.Duration
	if err == nil {
		totalBytes = copyRes.TotalBytes
		totalFiles = copyRes.TotalFiles
		duration = copyRes.Duration
	}

	return &FileCopyResult{
		CopiedFiles: totalFiles,
		Bytes:       totalBytes,
		Duration:    duration,
		Error:       err,
	}, err
}

func (t *LocalRunner) UsesNonAmbientCredentials() (bool, []string) {
	return false, nil
}

func (t *LocalRunner) Disconnect() error {
	return nil
}
