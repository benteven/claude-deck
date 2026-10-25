package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func State(url string) (string, error) {
	if !strings.HasPrefix(url, "https://") {
		return "", fmt.Errorf("not a PR url: %q", url)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", url, "--json", "state,isDraft")
	cmd.WaitDelay = 3 * time.Second
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var v struct {
		State   string `json:"state"`
		IsDraft bool   `json:"isDraft"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return "", err
	}
	switch {
	case v.State == "OPEN" && v.IsDraft:
		return "draft", nil
	case v.State == "OPEN":
		return "open", nil
	case v.State == "MERGED":
		return "merged", nil
	case v.State == "CLOSED":
		return "closed", nil
	}
	return "", fmt.Errorf("unknown PR state %q", v.State)
}
