//go:build unix

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCodexExecuteCleansUpDetachedMcpChildOnNormalExit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidFile := filepath.Join(dir, "mcp-child.pid")
	fakePath := writeFakeCodexAppServer(t, ""+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'`+"\n"+
		`read line`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thr-mcp-child"}}}'`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":3,"result":{}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thr-mcp-child","turn":{"id":"turn-mcp-child"}}}'`+"\n"+
		fmt.Sprintf(`(sleep 30) </dev/null >/dev/null 2>&1 & echo $! > %q`, pidFile)+"\n"+
		`echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thr-mcp-child","turn":{"id":"turn-mcp-child","status":"completed"}}}'`+"\n")

	result := executeFakeCodex(t, fakePath, ExecOptions{
		Cwd:                       dir,
		Timeout:                   5 * time.Second,
		SemanticInactivityTimeout: 5 * time.Second,
	})
	if result.Status != "completed" {
		t.Fatalf("expected completed, got status=%q error=%q", result.Status, result.Error)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read child pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse child pid %q: %v", data, err)
	}
	t.Cleanup(func() {
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Kill()
		}
	})

	waitProcessGone(t, pid)
}
