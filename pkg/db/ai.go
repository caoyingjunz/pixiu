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

// AIInterface groups all AI-related database accessors.
type AIInterface interface {
	Account() AIAccountInterface
	Conversation() AIConversationInterface
	ToolExecution() AIToolExecutionInterface
	ResponseExecution() AIResponseExecutionInterface
}

type ai struct {
	db *gorm.DB
}

func newAI(db *gorm.DB) AIInterface {
	return &ai{db: db}
}

func (a *ai) Account() AIAccountInterface {
	return newAIAccount(a.db)
}

func (a *ai) Conversation() AIConversationInterface {
	return newAIConversation(a.db)
}

func (a *ai) ToolExecution() AIToolExecutionInterface {
	return newAIToolExecution(a.db)
}

func (a *ai) ResponseExecution() AIResponseExecutionInterface {
	return newAIResponseExecution(a.db)
}
