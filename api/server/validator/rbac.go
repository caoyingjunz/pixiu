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
	"strconv"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/go-playground/validator/v10"
)

func init() {
	register(
		&objectValidator{pixiuValidator: newPixiuValidator("rbac_object", "对象类型不支持")},
		&operationValidator{pixiuValidator: newPixiuValidator("rbac_operation", "操作不支持")},
		&stringIDValidator{pixiuValidator: newPixiuValidator("rbac_sid", "不合法")},
	)
}

type objectValidator struct {
	pixiuValidator
}

func (ov *objectValidator) validate(fl validator.FieldLevel) bool {
	obj := fl.Field().Interface().(model.ObjectType)
	_, ok := model.ObjectTypeMap[obj]
	return ok
}

type operationValidator struct {
	pixiuValidator
}

func (ov *operationValidator) validate(fl validator.FieldLevel) bool {
	op := fl.Field().Interface().(model.Operation)
	_, ok := model.OperationMap[op]
	return ok
}

type stringIDValidator struct {
	pixiuValidator
}

func (sv *stringIDValidator) validate(fl validator.FieldLevel) bool {
	sid := fl.Field().String()
	if sid == "*" {
		return true
	}
	_, err := strconv.Atoi(sid)
	return err == nil
}
