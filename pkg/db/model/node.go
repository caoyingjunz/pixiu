package model

import "time"

type AuthModel int8

const (
	PASSWORD AuthModel = iota + 1
	PUBLICKEY
)

type Resize struct {
	Columns int
	Rows    int
}

type SSHClientConfig struct {
	AuthModel AuthModel
	Host      string
	Port      int
	User      string
	Password  string
	KeyPath   string
	Protocol  string
	Timeout   time.Duration
}
