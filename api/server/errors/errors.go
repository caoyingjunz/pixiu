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

package errors

import (
	"net/http"

	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Error struct {
	Code int
	Err  error
}

func (e Error) Error() string {
	return e.Err.Error()
}

func NewError(err error, code int) Error {
	return Error{
		Code: code,
		Err:  err,
	}
}

func IsError(err error) bool {
	_, ok := err.(Error)
	return ok
}

var (
	ErrInvalidRequest = Error{
		Code: http.StatusBadRequest,
		Err:  errors.ErrReqParams,
	}
	ErrServerInternal = Error{
		Code: http.StatusInternalServerError,
		Err:  errors.ErrInternal,
	}
	ErrUserNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrUserNotFound,
	}
	ErrUserExists = Error{
		Code: http.StatusConflict,
		Err:  errors.UserExistError,
	}
	ErrInvalidPassword = Error{
		Code: http.StatusUnauthorized,
		Err:  errors.ErrUserPassword,
	}
	ErrClusterNotFound = Error{
		Code: http.StatusNotFound,
		Err:  errors.ErrClusterNotFound,
	}
	ErrTenantExists = Error{
		Code: http.StatusConflict,
		Err:  errors.TenantExistError,
	}
)
