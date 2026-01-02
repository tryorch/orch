package runners

import (
	"context"
	"errors"
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
	return Capabilities{Exec: true, FileCopy: true, API: true}
}

func (t *LocalRunner) ValidateAndInitialize() error {
	return nil
}

func (t *LocalRunner) Exec(ctx context.Context, req ExecCommand) (*ExecResult, error) {
	cmd := exec.Command(req.Command[0], req.Command[1:]...)
	if req.Stdin != nil {
		cmd.Stdin = req.Stdin
	}
	if req.Stdout != nil {
		cmd.Stdout = req.Stdout
	}
	if req.Stderr != nil {
		cmd.Stderr = req.Stderr
	}

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

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
