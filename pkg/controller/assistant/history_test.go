package assistant

import (
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"strings"
	"testing"
)

func TestConversationHistoryRoundTrip(t *testing.T) {
	answer := strings.Repeat("完整回答", 3000)
	history, err := appendConversationHistory(nil, "第一问", answer)
	if err != nil {
		t.Fatal(err)
	}
	items, err := buildConversationInput(&model.Conversation{History: history}, "继续")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0]["role"] != "user" || items[1]["role"] != "assistant" || items[2]["role"] != "user" {
		t.Fatalf("unexpected history: %d items", len(items))
	}
	content := items[1]["content"].([]interface{})[0].(map[string]interface{})
	if content["text"] != answer {
		t.Fatal("answer truncated during persistence")
	}
	if _, err := buildConversationInput(&model.Conversation{History: "invalid"}, "继续"); err == nil {
		t.Fatal("invalid history silently discarded")
	}
}
