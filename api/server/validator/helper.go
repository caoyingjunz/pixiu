/*
Copyright 2024 The Pixiu Authors.

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

package validator

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

// TranslateError returns the translated message of the validation error.
func TranslateError(errs validator.ValidationErrors) string {
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Translate(tran)
	}

	return strings.Join(messages, "; ")
}
