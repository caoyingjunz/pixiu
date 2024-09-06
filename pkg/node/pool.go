package node

import (
	"bytes"
	"sync"
)

var BufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}
