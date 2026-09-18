package conversation

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/gin-gonic/gin"
)

type fakeConversations struct {
	db.ConversationInterface
	object      *model.Conversation
	deleted     bool
	selected    int64
	currentUser int64
}

func (f *fakeConversations) Get(context.Context, int64) (*model.Conversation, error) {
	return f.object, nil
}
func (f *fakeConversations) Delete(context.Context, int64) error { f.deleted = true; return nil }
func (f *fakeConversations) Current(_ context.Context, userID int64) (*model.Conversation, error) {
	f.currentUser = userID
	return f.object, nil
}
func (f *fakeConversations) Select(_ context.Context, userID, id int64) error {
	f.currentUser = userID
	f.selected = id
	return nil
}

type fakeAssistant struct {
	db.AssistantInterface
	conversations *fakeConversations
}

func (f *fakeAssistant) Conversation() db.ConversationInterface { return f.conversations }

type fakeFactory struct {
	db.ShareDaoFactory
	assistant *fakeAssistant
}

func (f *fakeFactory) Assistant() db.AssistantInterface { return f.assistant }

func TestConversationOwnership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name    string
		owner   *int64
		allowed bool
	}{
		{"own", ptr(7), true}, {"other user", ptr(8), false}, {"legacy unowned", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			httputils.SetUserToContext(ctx, &model.User{Model: pixiu.Model{Id: 7}})
			dao := &fakeConversations{object: &model.Conversation{Model: pixiu.Model{Id: 1}, UserId: test.owner}}
			c := &controller{factory: &fakeFactory{assistant: &fakeAssistant{conversations: dao}}}
			_, err := c.Get(ctx, 1)
			if (err == nil) != test.allowed {
				t.Fatalf("Get allowed=%v, error=%v", test.allowed, err)
			}
			err = c.Delete(ctx, 1)
			if (err == nil) != test.allowed || dao.deleted != test.allowed {
				t.Fatalf("Delete allowed=%v, deleted=%v, error=%v", test.allowed, dao.deleted, err)
			}
		})
	}
}
func ptr(id int64) *int64 { return &id }

func TestCurrentAndBlankSelection(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	httputils.SetUserToContext(ctx, &model.User{Model: pixiu.Model{Id: 7}})
	dao := &fakeConversations{selected: 12}
	c := &controller{factory: &fakeFactory{assistant: &fakeAssistant{conversations: dao}}}
	current, err := c.Current(ctx)
	if err != nil || current != nil || dao.currentUser != 7 {
		t.Fatalf("unexpected blank result: %v %v", current, err)
	}
	if err := c.Select(ctx, 0); err != nil || dao.selected != 0 {
		t.Fatalf("blank selection failed: %v", err)
	}
	if err := c.Select(ctx, -1); err == nil {
		t.Fatal("negative ID accepted")
	}
}

func TestExecutionsRejectOtherUsersAndUnownedHistory(t *testing.T) {
	for _, owner := range []*int64{ptr(8), nil} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		httputils.SetUserToContext(ctx, &model.User{Model: pixiu.Model{Id: 7}})
		dao := &fakeConversations{object: &model.Conversation{Model: pixiu.Model{Id: 1}, UserId: owner}}
		c := &controller{factory: &fakeFactory{assistant: &fakeAssistant{conversations: dao}}}
		// Execution DAO is deliberately absent: unauthorized requests must never reach it.
		if _, err := c.Executions(ctx, 1, types.ListOptions{}); err == nil {
			t.Fatal("unauthorized execution history exposed")
		}
	}
}
