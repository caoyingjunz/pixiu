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

package sqlbackup

import (
	"errors"
	"io"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	MaxBytes      = 128 << 20
	MaxTables     = 200
	Timeout       = 5 * time.Minute
	previewBytes  = 12000
	footerReserve = 512
)

const sizeNote = "\n-- Backup truncated: file size limit reached.\n"

type Writer struct {
	out     io.Writer
	limit   int64
	size    int64
	preview strings.Builder
}

func NewWriter(out io.Writer) *Writer {
	return &Writer{out: out, limit: MaxBytes}
}

func (w *Writer) Statement(text string) (bool, error) {
	if int64(len(text)) > w.limit-footerReserve-w.size {
		return false, nil
	}
	return true, w.write(text)
}

func (w *Writer) write(text string) error {
	return w.writeBytes([]byte(text))
}

func (w *Writer) writeBytes(p []byte) error {
	n, err := w.out.Write(p)
	if err != nil {
		return err
	}
	if n != len(p) {
		return io.ErrShortWrite
	}
	w.size += int64(n)
	if left := previewBytes - w.preview.Len(); left > 0 {
		w.preview.Write(p[:min(left, n)])
	}
	return nil
}

func (w *Writer) Finish(summary *types.SQLBackupSummary, footer string, sizeLimited bool) error {
	if sizeLimited {
		footer = sizeNote + footer
		summary.Truncated = true
	}
	if int64(len(footer)) > w.limit-w.size || w.size == 0 {
		return errors.New("SQL backup has no writable content")
	}
	if err := w.write(footer); err != nil {
		return err
	}
	summary.SizeBytes = w.size
	summary.Preview = strings.ToValidUTF8(w.preview.String(), "")
	if w.size > previewBytes {
		summary.Preview += "\n-- Preview truncated."
	}
	return nil
}

type Inserts struct {
	out    *Writer
	prefix string
	buf    []byte
	rows   int
	size   int
}

func NewInserts(out *Writer, prefix string) *Inserts {
	return &Inserts{out: out, prefix: prefix}
}

func (b *Inserts) RowBytes(tuple []byte) (bool, error) {
	return b.add(len(tuple), func() {
		b.buf = append(b.buf, tuple...)
	})
}

func (b *Inserts) add(n int, appendTuple func()) (bool, error) {
	if b.rows > 0 && (b.rows >= 50 || b.size+n > 64<<10) {
		if ok, err := b.Flush(); !ok || err != nil {
			return ok, err
		}
	}
	if b.rows > 0 {
		b.buf = append(b.buf, ',', '\n')
	}
	appendTuple()
	b.rows++
	b.size += n + 2
	if b.size >= 64<<10 {
		return b.Flush()
	}
	return true, nil
}

func (b *Inserts) Flush() (bool, error) {
	if b.rows == 0 {
		return true, nil
	}
	if int64(len(b.prefix)+len(b.buf)+2) > b.out.limit-footerReserve-b.out.size {
		b.reset()
		return false, nil
	}
	if err := b.out.write(b.prefix); err != nil {
		return false, err
	}
	if err := b.out.writeBytes(b.buf); err != nil {
		return false, err
	}
	if err := b.out.write(";\n"); err != nil {
		return false, err
	}
	b.reset()
	return true, nil
}

func (b *Inserts) reset() {
	b.buf = b.buf[:0]
	b.rows = 0
	b.size = 0
}
