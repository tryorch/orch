package runners

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"orch.io/pkg/utils"
)

type SSHRunnerConfig struct {
	Host string `yaml:"host" mapstructure:"host"`
	Port int    `yaml:"port" mapstructure:"port"`
	User string `yaml:"user" mapstructure:"user"`
	Auth struct {
		Method   string `yaml:"method" mapstructure:"method"`
		Password string `yaml:"password,omitempty" mapstructure:"password"`
		KeyPath  string `yaml:"key_path,omitempty" mapstructure:"key_path"`
	} `yaml:"auth" mapstructure:"auth"`
	HostKey SSHHostKeyConfig `yaml:"host_key" mapstructure:"host_key"`
}

type SSHHostKeyConfig struct {
	KnownHosts string `yaml:"known_hosts,omitempty" mapstructure:"known_hosts"`
	Insecure   bool   `yaml:"insecure,omitempty" mapstructure:"insecure"`
}

type SSHRunner struct {
	name   string
	config SSHRunnerConfig
	env    map[string]string

	client *ssh.Client // managed SSH client
}

func (t *SSHRunner) Name() string {
	return t.name
}

func (t *SSHRunner) Type() RunnerType {
	return RunnerTypeSSH
}

func (t *SSHRunner) Capabilities() Capabilities {
	return Capabilities{Exec: true, FileCopy: true}
}

func (t *SSHRunner) ValidateAndInitialize() error {
	if t.client != nil {
		return nil // already connected
	}

	if t.config.Host == "" {
		return errors.New("host required")
	}

	if t.config.User == "" {
		return errors.New("user required")
	}

	if t.config.Auth.Method == "password" && t.config.Auth.Password == "" {
		return errors.New("password authentication requires a password")
	}

	if t.config.Auth.Method == "key" && t.config.Auth.KeyPath == "" {
		return errors.New("key authentication requires a key path")
	}

	var auth []ssh.AuthMethod
	if t.config.Auth.Method == "password" {
		auth = append(auth, ssh.Password(t.config.Auth.Password))
	} else if t.config.Auth.Method == "key" {
		key, err := os.ReadFile(t.config.Auth.KeyPath)
		if err != nil {
			return fmt.Errorf("failed to read private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	} else {
		return fmt.Errorf("unsupported auth method: %s", t.config.Auth.Method)
	}

	hostKeyCallback, err := t.hostKeyCallback()
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", t.config.Host, t.config.Port)
	config := &ssh.ClientConfig{
		User:            t.config.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to dial SSH: %w", err)
	}
	t.client = client

	return nil
}

func (t *SSHRunner) hostKeyCallback() (ssh.HostKeyCallback, error) {
	cfg := t.config.HostKey
	methods := 0
	if cfg.KnownHosts != "" {
		methods++
	}
	if cfg.Insecure {
		methods++
	}

	if methods == 0 {
		return nil, errors.New("ssh host_key is required; configure host_key.known_hosts or host_key.insecure")
	}
	if methods > 1 {
		return nil, errors.New("ssh host_key must set exactly one verification method")
	}

	if cfg.Insecure {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	knownHostsPath, err := expandUserPath(cfg.KnownHosts)
	if err != nil {
		return nil, err
	}
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load known_hosts file %q: %w", cfg.KnownHosts, err)
	}
	return callback, nil
}

func expandUserPath(path string) (string, error) {
	if path == "" || path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		if path == "" || path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func (t *SSHRunner) Exec(ctx context.Context, req ExecCommand) (*ExecResult, error) {
	if !t.Capabilities().Exec {
		return nil, errors.New("exec not supported. runner does not support Exec")
	}

	session, err := t.client.NewSession()
	if err != nil {
		return nil, err
	}

	defer func(session *ssh.Session) {
		err := session.Close()
		if err != nil {

		}
	}(session)

	if req.Stdin != nil {
		session.Stdin = req.Stdin
	}
	var stdout bytes.Buffer
	if req.Stdout != nil {
		session.Stdout = io.MultiWriter(req.Stdout, &stdout)
	} else {
		session.Stdout = &stdout
	}
	var stderr bytes.Buffer
	if req.Stderr != nil {
		session.Stderr = io.MultiWriter(req.Stderr, &stderr)
	} else {
		session.Stderr = &stderr
	}

	cmd := buildSSHCommand(t.env, req)

	start := time.Now()
	err = session.Run(cmd)
	duration := time.Since(start)
	ensureExecWriterLineEnds(req.Stdout, req.Stderr)

	exitCode := 0
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitStatus()
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

func buildSSHCommand(baseEnv map[string]string, req ExecCommand) string {
	parts := make([]string, 0, len(req.Command))
	for _, arg := range req.Command {
		parts = append(parts, shellQuote(arg))
	}
	command := strings.Join(parts, " ")

	env := mergeEnv(baseEnv, req.Env)
	if len(env) > 0 {
		keys := make([]string, 0, len(env))
		for key := range env {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		envParts := make([]string, 0, len(keys)+2)
		envParts = append(envParts, "env")
		for _, key := range keys {
			envParts = append(envParts, shellQuote(key+"="+env[key]))
		}
		envParts = append(envParts, command)
		command = strings.Join(envParts, " ")
	}

	if req.WorkingDir != "" {
		command = "cd " + shellQuote(req.WorkingDir) + " && " + command
	}

	return command
}

func mergeEnv(envs ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, env := range envs {
		for key, value := range env {
			merged[key] = value
		}
	}
	return merged
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func (t *SSHRunner) CopyFile(ctx context.Context, req FileCopyRequest) (*FileCopyResult, error) {
	if !t.Capabilities().FileCopy {
		return nil, errors.New("FileCopy not supported")
	}

	sftpClient, err := sftp.NewClient(t.client)
	if err != nil {
		return nil, err
	}
	defer func(sftpClient *sftp.Client) {
		err := sftpClient.Close()
		if err != nil {

		}
	}(sftpClient)

	var srcFS, dstFS utils.FSWithPath
	if req.ToRunner {
		srcFS = utils.FSWithPath{FS: &utils.LocalFS{}, Path: req.Source}
		dstFS = utils.FSWithPath{FS: &utils.SFTPFS{SftpClient: sftpClient}, Path: req.Destination}
	} else {
		srcFS = utils.FSWithPath{FS: &utils.SFTPFS{SftpClient: sftpClient}, Path: req.Source}
		dstFS = utils.FSWithPath{FS: &utils.LocalFS{}, Path: req.Destination}
	}

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

func (t *SSHRunner) UsesNonAmbientCredentials() (bool, []string) {
	var creds []string
	if t.config.Auth.Method == "password" {
		creds = append(creds, "SSH password")
	} else if t.config.Auth.Method == "key" {
		creds = append(creds, fmt.Sprintf("SSH key at %s", t.config.Auth.KeyPath))
	}
	return len(creds) > 0, creds
}

func (t *SSHRunner) Disconnect() error {
	if t.client != nil {
		return t.client.Close()
	}
	return nil
}
