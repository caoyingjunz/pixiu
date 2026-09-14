package assistant

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

type timeoutAfterData struct {
	io.Reader
	readsAfterEnd int
}

func (r *timeoutAfterData) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		r.readsAfterEnd++
		return 0, errors.New("connection timed out after completion")
	}
	return n, err
}
func TestResponsesCompletionDoesNotWaitForEOF(t *testing.T) {
	r := &timeoutAfterData{Reader: strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"r1\"}}\n\n")}
	_, text, err := parseSSEResponseStream(r, func(*types.AIStreamEvent) error { return nil })
	if err != nil || text != "hello" || r.readsAfterEnd != 0 {
		t.Fatalf("text=%q err=%v extra reads=%d", text, err, r.readsAfterEnd)
	}
}
func TestChatDoneDoesNotWaitForEOF(t *testing.T) {
	r := &timeoutAfterData{Reader: strings.NewReader("data: {\"id\":\"r1\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n")}
	result, err := parseChatCompletionsSSE(r, func(*types.AIStreamEvent) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "hello" || r.readsAfterEnd != 0 {
		t.Fatalf("text=%q extra reads=%d", result.Text, r.readsAfterEnd)
	}
}
func TestResponsesNestedErrors(t *testing.T) {
	for _, test := range []struct{ payload, want string }{
		{`{"type":"response.failed","response":{"error":{"message":"quota exceeded"}}}`, "quota exceeded"},
		{`{"type":"error","error":{"message":"upstream unavailable"}}`, "upstream unavailable"},
		{`{"type":"response.incomplete","response":{"incomplete_details":{"reason":"max_output_tokens"}}}`, "max_output_tokens"},
	} {
		_, _, err := parseSSEResponseStream(strings.NewReader("data: "+test.payload+"\n\n"), func(*types.AIStreamEvent) error { return nil })
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("want %q, got %v", test.want, err)
		}
	}
}
