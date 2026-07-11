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

package db

import "gorm.io/gorm"

// AssistantInterface groups all assistant-related database accessors.
type AssistantInterface interface {
	Provider() AIProviderInterface
	Conversation() ConversationInterface
	Execution() ExecutionInterface
	Message() MessageInterface
}

type assistant struct {
	db *gorm.DB
}

func newAssistant(db *gorm.DB) AssistantInterface {
	return &assistant{db: db}
}

func (a *assistant) Provider() AIProviderInterface {
	return newAIProvider(a.db)
}

func (a *assistant) Conversation() ConversationInterface {
	return newConversation(a.db)
}

func (a *assistant) Execution() ExecutionInterface {
	return newExecution(a.db)
}

func (a *assistant) Message() MessageInterface {
	return newMessage(a.db)
}
