package util

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

func RunCmdWithStdPipes(cmd *exec.Cmd) error {
	errChan := make(chan error)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	go func() {
		if _, err := io.Copy(os.Stdout, stdout); err != nil {
			errChan <- err
		}
	}()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	go func() {
		if written, err := io.Copy(os.Stderr, stderr); err != nil {
			errChan <- err

			if written > 0 {
				errChan <- fmt.Errorf("%d bytes written to stderr", written) // TODO: wrap err
			}
		}
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	close(errChan)
	if err := <-errChan; err != nil {
		return err
	}

	return nil
}
