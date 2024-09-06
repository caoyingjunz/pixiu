package node

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"io"
	"k8s.io/klog/v2"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

const (
	MsgData   = '1'
	MsgResize = '2'
)

type Turn struct {
	StdinPipe io.WriteCloser
	Session   *ssh.Session
	WsConn    *websocket.Conn
}

func NewTurn(wsConn *websocket.Conn, sshClient *ssh.Client) (*Turn, error) {
	sess, err := sshClient.NewSession()
	if err != nil {
		return nil, err
	}

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return nil, err
	}

	turn := &Turn{StdinPipe: stdinPipe, Session: sess, WsConn: wsConn}
	sess.Stdout = turn
	sess.Stderr = turn

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // disable echo
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	if err := sess.RequestPty("xterm", 150, 30, modes); err != nil {
		return nil, err
	}
	if err := sess.Shell(); err != nil {
		return nil, err
	}

	return turn, nil
}

func (t *Turn) Write(p []byte) (n int, err error) {
	writer, err := t.WsConn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	defer writer.Close()

	return writer.Write(p)
}
func (t *Turn) Close() error {
	if t.Session != nil {
		t.Session.Close()
	}
	return t.WsConn.Close()
}

func (t *Turn) Read(p []byte) (n int, err error) {
	for {
		msgType, reader, err := t.WsConn.NextReader()
		if err != nil {
			return 0, err
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		return reader.Read(p)
	}
}

func (t *Turn) StartLoopRead(ctx context.Context, wg *sync.WaitGroup, logBuff *bytes.Buffer) {
	defer wg.Done()
	err := t.loopRead(logBuff, ctx)
	if err != nil {
		klog.Errorf("LoopRead exit, err:%s", err)
	}
}

func (t *Turn) loopRead(logBuff *bytes.Buffer, context context.Context) error {
	for {
		select {
		case <-context.Done():
			return errors.New("LoopRead exit")
		default:
			_, wsData, err := t.WsConn.ReadMessage()
			if err != nil {
				return fmt.Errorf("reading webSocket message err:%s", err)
			}
			body := decode(wsData[1:])

			switch wsData[0] {
			case MsgResize:
				if err := t.resizeDo(body); err != nil {
					return err
				}
			case MsgData:
				if err := t.dataDo(body, logBuff); err != nil {
					return err
				}
			}
		}
	}
}

func (t *Turn) dataDo(body []byte, logBuff *bytes.Buffer) error {
	if _, err := t.StdinPipe.Write(body); err != nil {
		return fmt.Errorf("StdinPipe write err:%s", err)
	}

	if _, err := logBuff.Write(body); err != nil {
		return fmt.Errorf("logBuff write err:%s", err)
	}
	return nil
}

func (t *Turn) resizeDo(body []byte) error {
	var args model.Resize
	err := json.Unmarshal(body, &args)
	if err != nil {
		return fmt.Errorf("ssh pty resize windows err:%s", err)
	}

	if args.Columns > 0 && args.Rows > 0 {
		if err := t.Session.WindowChange(args.Rows, args.Columns); err != nil {
			return fmt.Errorf("ssh pty resize windows err:%s", err)
		}
	}
	return nil
}

func (t *Turn) sessionWait() error {
	if err := t.Session.Wait(); err != nil {
		return err
	}
	return nil
}

func (t *Turn) StartSessionWait(wg *sync.WaitGroup) {
	defer wg.Done()
	err := t.sessionWait()
	if err != nil {
		klog.Errorf("SessionWait exit, err:%s", err)
	}
}

func decode(p []byte) []byte {
	decodeString, _ := base64.StdEncoding.DecodeString(string(p))
	return decodeString
}
