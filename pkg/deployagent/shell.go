/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deployagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/agentshell"
)

// StartShellServer 在本地 Unix socket 上提供 PTY Shell 服务。
// 服务端通过反向隧道 dial 该 socket。监听成功后立即返回，服务在后台运行，
// ctx 取消时关闭监听并清理 socket 文件。
func StartShellServer(ctx context.Context, cfg ShellConfig) error {
	// 路径固定：服务端始终拨号 agentshell.SocketPath。
	socket := agentshell.SocketPath

	// 目录 0700：即使 socket 尚未 chmod，本机非 root 用户也无法穿越目录连接。
	socketDir := filepath.Dir(socket)
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		return fmt.Errorf("create shell socket dir: %w", err)
	}
	// 纵深防御：防止 /run/pixiu 被本地用户预置为可写目录或符号链接，劫持 shell socket。
	if err := validateShellSocketDir(socketDir, true); err != nil {
		return fmt.Errorf("validate shell socket dir %s: %w", socketDir, err)
	}
	if err := validateShellSocketDir(filepath.Dir(socketDir), false); err != nil {
		return fmt.Errorf("validate shell socket parent dir: %w", err)
	}
	// 已有实例探测：能连上说明服务已存在，不再重复监听。
	if conn, err := net.DialTimeout("unix", socket, time.Second); err == nil {
		_ = conn.Close()
		return fmt.Errorf("shell socket %s already in use", socket)
	}
	// 连接失败视为上次残留的 socket 文件，清理后重建。
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale shell socket: %w", err)
	}

	ln, err := net.Listen("unix", socket)
	if err != nil {
		return fmt.Errorf("listen shell socket: %w", err)
	}
	// root-only，防止本机普通用户直接连入获得高权限 shell。
	if err := os.Chmod(socket, 0o600); err != nil {
		_ = ln.Close()
		return fmt.Errorf("chmod shell socket: %w", err)
	}

	go func() {
		<-ctx.Done()
		_ = ln.Close()
		_ = os.Remove(socket)
	}()

	shell := cfg.Command
	if shell == "" {
		shell = defaultShellCommand()
	}
	klog.Infof("deploy-agent shell server listening on %s (shell=%s)", socket, shell)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
					return
				}
				klog.Errorf("shell accept failed: %v", err)
				continue
			}
			go serveShellConn(ctx, conn, shell)
		}
	}()

	return nil
}

// validateShellSocketDir 校验 shell socket 目录：必须是真实目录（非符号链接），
// owner 必须是当前进程用户，且组/其他不可写。tighten 为 true 时尝试收紧为 0700 并复查。
// 目录被其他用户预建时，即使权限位为 0755，owner 仍可在目录内替换 socket，故一并拒绝。
func validateShellSocketDir(dir string, tighten bool) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	// Lstat 对符号链接返回链接自身，IsDir 为 false，天然拒绝。
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a real directory", dir)
	}
	// deploy-agent 仅支持 unix；Stat_t 断言失败时跳过 owner 校验。
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Geteuid() {
		return fmt.Errorf("%s is owned by uid %d, expected euid %d", dir, st.Uid, os.Geteuid())
	}
	if fi.Mode().Perm()&0o022 == 0 {
		return nil
	}
	if tighten {
		if chmodErr := os.Chmod(dir, 0o700); chmodErr != nil {
			klog.Warningf("cannot tighten shell socket dir %s permissions: %v", dir, chmodErr)
		} else {
			fi, err = os.Lstat(dir)
			if err != nil {
				return err
			}
			if fi.Mode().Perm()&0o022 == 0 {
				return nil
			}
		}
	}
	return fmt.Errorf("%s is group/world-writable (mode %04o)", dir, fi.Mode().Perm())
}

// serveShellConn 为一个隧道连接启动 PTY shell 并转发帧。
func serveShellConn(ctx context.Context, conn net.Conn, shell string) {
	defer func() { _ = conn.Close() }()
	klog.Infof("deploy-agent shell session started (shell=%s)", shell)
	defer klog.Infof("deploy-agent shell session ended")

	// #nosec G204 -- shell 路径来自本机配置，非远端输入。
	cmd := exec.Command(shell, "-i")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		klog.Errorf("failed to start shell %s: %v", shell, err)
		return
	}

	var closeOnce sync.Once
	closePTY := func() {
		closeOnce.Do(func() { _ = ptmx.Close() })
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		closePTY()
		_ = cmd.Wait()
	}()

	// ctx 取消（进程退出）时终止当前会话。
	watchDone := make(chan struct{})
	defer close(watchDone)
	go func() {
		select {
		case <-ctx.Done():
			closePTY()
			_ = conn.Close()
		case <-watchDone:
		}
	}()

	// PTY 输出 -> 帧。唯一的帧写入方，无需额外加锁。
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				if werr := agentshell.WriteFrame(conn, agentshell.FrameData, buf[:n]); werr != nil {
					break
				}
			}
			if rerr != nil {
				break
			}
		}
		// shell 退出或连接写失败：关闭连接以唤醒帧读取循环。
		closePTY()
		_ = conn.Close()
	}()

	// 帧 -> PTY。
	for {
		typ, payload, rerr := agentshell.ReadFrame(conn)
		if rerr != nil {
			return
		}
		switch typ {
		case agentshell.FrameData:
			if _, werr := ptmx.Write(payload); werr != nil {
				return
			}
		case agentshell.FrameResize:
			var resize agentshell.Resize
			if uerr := json.Unmarshal(payload, &resize); uerr != nil {
				klog.Warningf("invalid resize payload: %v", uerr)
				continue
			}
			// 限制在 uint16 可表示范围，避免大值转换溢出回绕。
			if resize.Columns <= 0 || resize.Columns > 65535 || resize.Rows <= 0 || resize.Rows > 65535 {
				klog.Warningf("ignoring out-of-range resize: columns=%d rows=%d", resize.Columns, resize.Rows)
				continue
			}
			_ = pty.Setsize(ptmx, &pty.Winsize{
				Rows: uint16(resize.Rows),
				Cols: uint16(resize.Columns),
			})
		default:
			klog.Warningf("unknown shell frame type %d", typ)
		}
	}
}
