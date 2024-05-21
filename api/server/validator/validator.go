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
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zt "github.com/go-playground/validator/v10/translations/zh"
)

type customValidator interface {
	getTag() string
	translateError(ut ut.Translator) error
	translate(ut ut.Translator, fe validator.FieldError) string

	// Should be implemented by the custom validator.
	validate(fl validator.FieldLevel) bool
}

var tran ut.Translator
var customValidators []customValidator

// register adds a new custom validator to the validator list
func register(validators ...customValidator) {
	customValidators = append(customValidators, validators...)
}

func init() {
	_zh := zh.New() // default is Chinese
	uni := ut.New(_zh, _zh)
	tran, _ = uni.GetTranslator("zh")

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = zt.RegisterDefaultTranslations(v, tran)

		for _, c := range customValidators {
			_ = v.RegisterValidation(c.getTag(), c.validate)
			_ = v.RegisterTranslation(c.getTag(), tran, c.translateError, c.translate)
		}
	}
}

type pixiuValidator struct {
	tag string
	err string
}

func newPixiuValidator(tag, err string) pixiuValidator {
	return pixiuValidator{
		tag: tag,
		err: err,
	}
}

func (c pixiuValidator) getTag() string {
	return c.tag
}

func (c pixiuValidator) translateError(ut ut.Translator) error {
	return ut.Add(c.tag, "{0}"+c.err, true)
}

func (c pixiuValidator) translate(ut ut.Translator, fe validator.FieldError) string {
	t, _ := ut.T(c.tag, fe.Field())
	return t
}
