package deck

import (
	"fmt"
	"os/exec"
	"runtime"
)

func trashCommand(paths []string) (*exec.Cmd, error) {
	if runtime.GOOS == "darwin" {
		return exec.Command("/usr/bin/trash", paths...), nil
	}
	if gio, err := exec.LookPath("gio"); err == nil {
		return exec.Command(gio, append([]string{"trash"}, paths...)...), nil
	}
	if put, err := exec.LookPath("trash-put"); err == nil {
		return exec.Command(put, paths...), nil
	}
	return nil, fmt.Errorf("install gio or trash-cli to delete")
}
