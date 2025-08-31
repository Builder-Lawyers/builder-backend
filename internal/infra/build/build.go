package build

import (
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type TemplateBuild struct {
}

func NewTemplateBuild() *TemplateBuild {
	return &TemplateBuild{}
}

// To run this, container running application has to have npm installed and "npm i -g pnpm"
func (b *TemplateBuild) RunFrontendBuild(path string) (string, error) {
	templatesRootDir := filepath.Dir(path)
	if len(path) != 0 {
		log.Println("Valid dir")
	}
	// TODO: first check if node modules exist, then run build or first install
	build := createProcess(path, "npm run build")
	err := build.Start()
	if err != nil {
		slog.Error("failed to start npm run build: %v", "build", err)
	}

	log.Printf("npm run build started with PID %d", build.Process.Pid)

	err = build.Wait()
	if err != nil {
		slog.Error("npm run build exited with error: %v", "build", err)
		installDeps := createProcess(templatesRootDir, "pnpm i")

		err = installDeps.Start()
		if err != nil {
			slog.Error("failed to install dependencies: %v", "build", err)
			return "", err
		}
		err = installDeps.Wait()
		if err != nil {
			slog.Error("failed to install dependencies: %v", "build", err)
			return "", err
		}

		build = createProcess(path, "npm run build")
		err = build.Start()
		if err != nil {
			slog.Error("failed to start npm run build: %v", "build", err)
			return "", err
		}

		log.Printf("npm run build started with PID %d", build.Process.Pid)
		err = build.Wait()
		if err != nil {
			slog.Error("fatal error", "build", err)
			return "", err
		}
	}

	return path + "/dist", nil
}

func createProcess(dir string, command string) *exec.Cmd {
	log.Println(command)
	params := strings.Split(command, " ")
	proc := exec.Command(params[0], params[1:]...)
	proc.Dir = dir
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Stdin = os.Stdin
	return proc
}
