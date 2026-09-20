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

package auth

import goerrors "errors"

var (
	errCodeTooFrequent = goerrors.New("registration code sent too frequently")
	errCodeInvalid     = goerrors.New("invalid registration code")
	errCodeExpired     = goerrors.New("registration code expired")
	errCodeUsed        = goerrors.New("registration code already used")
	errCodeAttempts    = goerrors.New("too many registration code attempts")
	errEmailExists     = goerrors.New("registration email already exists")
	errUserExists      = goerrors.New("registration user already exists")
	errRoleUnavailable = goerrors.New("registration role unavailable")
	errRoleConflict    = goerrors.New("multiple registration roles found")
)
