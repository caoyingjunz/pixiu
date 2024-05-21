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
	"github.com/go-playground/validator/v10"

	"github.com/caoyingjunz/pixiu/pkg/util"
)

func init() {
	register(
		&passwordValidator{pixiuValidator: newPixiuValidator("password", "密码不符合要求，至少包含一个大写字母、一个小写字母、一个数字")},
	)
}

// passwordValidator is a customized validator for validating user password.
type passwordValidator struct {
	pixiuValidator
}

// validate validates the password in request.
func (pv *passwordValidator) validate(fl validator.FieldLevel) bool {
	return util.ValidateStrongPassword(fl.Field().String())
}
