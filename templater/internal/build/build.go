package build

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func RunFrontendBuild(dir string) {
	if len(dir) != 0 {
		log.Println("Valid dir")
	}
	build := createProcess(dir, "npm run build")
	err := build.Start()
	if err != nil {
		log.Fatalf("Failed to start npm run build: %v", err)
	}

	log.Printf("npm run build started with PID %d", build.Process.Pid)

	err = build.Wait()
	if err != nil {
		log.Printf("npm run build exited with error: %v", err)
		installDeps := createProcess(dir, "npm i")

		err = installDeps.Start()
		if err != nil {
			log.Fatalf("Failed to start npm run build: %v", err)
		}
		log.Printf("npm run build started with PID %d", installDeps.Process.Pid)
		err = installDeps.Wait()
		if err != nil {
			log.Fatalf("Failed to install dependencies")
		}
		build = createProcess(dir, "npm run build")
		err = build.Start()
		if err != nil {
			log.Fatalf("Failed to start npm run build: %v", err)
		}

		log.Printf("npm run build started with PID %d", build.Process.Pid)

		err = build.Wait()
		if err != nil {
			fmt.Println("Fatal error", err)
		}
	}
}

func createProcess(dir string, command string) *exec.Cmd {
	fmt.Println(command)
	params := strings.Split(command, " ")
	proc := exec.Command(params[0], params[1:]...)
	proc.Dir = dir
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Stdin = os.Stdin
	return proc
}
