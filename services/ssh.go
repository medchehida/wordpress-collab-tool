package services

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"wordpress-collab-tool/config" // Import the config package
	"wordpress-collab-tool/utils"  // Import the utils package for logging
)

// GetSSHClient establishes an SSH connection to the remote server.
func GetSSHClient(cfg *config.Config) (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: cfg.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.SSHPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Implement proper host key verification
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", cfg.SSHHost), sshConfig)
	if err != nil {
		utils.LogError("Failed to dial SSH: %v", err)
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}
	utils.LogInfo("SSH connection established to %s", cfg.SSHHost)
	return client, nil
}

// RunSSHCommand executes a command on the remote server via SSH.
func RunSSHCommand(client *ssh.Client, command string) (string, string, error) {
	session, err := client.NewSession()
	if err != nil {
		utils.LogError("Failed to create SSH session: %v", err)
		return "", "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf strings.Builder
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	utils.LogInfo("Executing SSH command: %s", command)
	err = session.Run(command)
	if err != nil {
		utils.LogError("SSH command failed: %v, stdout: %s, stderr: %s", err, stdoutBuf.String(), stderrBuf.String())
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("SSH command failed: %w", err)
	}
	utils.LogInfo("SSH command executed successfully. Stdout: %s, Stderr: %s", stdoutBuf.String(), stderrBuf.String())
	return stdoutBuf.String(), stderrBuf.String(), nil
}

// GetSFTPClient creates an SFTP client from an SSH client.
func GetSFTPClient(client *ssh.Client) (*sftp.Client, error) {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		utils.LogError("Failed to create SFTP client: %v", err)
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}
	utils.LogInfo("SFTP client created.")
	return sftpClient, nil
}

// UploadFile uploads a file to the remote server via SFTP.
func UploadFile(sftpClient *sftp.Client, remotePath string, content io.Reader) error {
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		utils.LogError("Failed to create remote file %s: %v", remotePath, err)
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, content)
	if err != nil {
		utils.LogError("Failed to write to remote file %s: %v", remotePath, err)
		return fmt.Errorf("failed to write to remote file: %w", err)
	}
	utils.LogInfo("File uploaded to %s", remotePath)
	return nil
}