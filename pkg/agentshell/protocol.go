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

// Package agentshell defines the protocol shared by the Pixiu server and the
// deploy agent for interactive host shells over the reverse tunnel.
//
// Frames are exchanged over the tunnel connection dialed by the server to the
// agent's local Unix socket:
//
//	[1B type][4B big-endian payload length][payload]
package agentshell

import (
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	// SocketDir is the agent-local directory owning the PTY shell socket.
	// It is created with 0700 permissions so non-root local users cannot
	// reach the socket even before it is chmod-ed.
	SocketDir = "/run/pixiu/deploy-agent"
	// SocketPath is the default Unix socket path of the agent-local PTY shell
	// service (on the agent host).
	SocketPath = SocketDir + "/shell.sock"

	// FrameData carries raw terminal input/output bytes.
	// 与既有终端协议（pkg/types MsgData = '1'）保持一致，使用字符而非数值。
	FrameData byte = '1'
	// FrameResize carries a JSON-encoded Resize payload.
	// 与既有终端协议（pkg/types MsgResize = '2'）保持一致。
	FrameResize byte = '2'

	// MaxFrameSize is the maximum payload size of a single frame (1MB).
	MaxFrameSize = 1 << 20

	sessionKeyPrefix = "agent:"
	frameHeaderSize  = 5
)

// SessionKey returns the remotedialer client key for a deploy agent.
func SessionKey(agentID int64) string {
	return sessionKeyPrefix + strconv.FormatInt(agentID, 10)
}

// IsAgentSessionKey reports whether key is a deploy agent session key.
func IsAgentSessionKey(key string) bool {
	if !strings.HasPrefix(key, sessionKeyPrefix) {
		return false
	}
	if _, err := strconv.ParseInt(strings.TrimPrefix(key, sessionKeyPrefix), 10, 64); err != nil {
		return false
	}
	return true
}

// Resize is the terminal window size payload carried by FrameResize.
type Resize struct {
	Columns int `json:"columns"`
	Rows    int `json:"rows"`
}

// WriteFrame writes a single frame to w.
func WriteFrame(w io.Writer, typ byte, payload []byte) error {
	if len(payload) > MaxFrameSize {
		return fmt.Errorf("agentshell: payload too large: %d > %d", len(payload), MaxFrameSize)
	}

	var header [frameHeaderSize]byte
	header[0] = typ
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))

	if err := writeAll(w, header[:]); err != nil {
		return err
	}
	return writeAll(w, payload)
}

// ReadFrame reads a single frame from r.
func ReadFrame(r io.Reader) (byte, []byte, error) {
	var header [frameHeaderSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}

	length := binary.BigEndian.Uint32(header[1:])
	if length > MaxFrameSize {
		return 0, nil, fmt.Errorf("agentshell: frame too large: %d > %d", length, MaxFrameSize)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return header[0], payload, nil
}

func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}
