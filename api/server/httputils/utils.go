/*
Copyright 2021 The Pixiu Authors.

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

package httputils

import (
	"fmt"
	"net"
	"strings"
	"time"

	geoip2db "github.com/cc14514/go-geoip2-db"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/mssola/user_agent"
	"io/ioutil"
)

type Claims struct {
	jwt.RegisteredClaims

	Id   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// GenerateToken 生成 token
func GenerateToken(uid int64, name string, jwtKey []byte) (string, error) {
	// Generate jwt, 临时有效期 360 分钟
	nowTime := time.Now()
	expiresTime := nowTime.Add(360 * time.Minute)
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresTime), // 过期时间
			IssuedAt:  jwt.NewNumericDate(nowTime),     // 签发时间
			NotBefore: jwt.NewNumericDate(nowTime),     // 生效时间
		},
		Id:   uid,
		Name: name,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func ParseToken(tokenStr string, jwtKey []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("failed to parse invailed token")
}

// ReadFile 从请求中获取指定文件内容
func ReadFile(c *gin.Context, f string) ([]byte, error) {
	fileHeader, err := c.FormFile(f)
	if err != nil {
		return nil, err
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ioutil.ReadAll(file)
}

// GetCityByIp 通过ip获取城市信息 类库
func GetCityByIp(ip string) string {
	if ip == "" {
		return ""
	}
	if ip == "[::1]" || ip == "127.0.0.1" {
		return "内网IP"
	}
	db, err := geoip2db.NewGeoipDbByStatik()
	defer db.Close()
	if err != nil {
		return ""
	}
	record, err := db.City(net.ParseIP(ip))
	if err != nil {
		return ""
	}
	city := fmt.Sprintf("%s %s", record.Subdivisions[0].Names["zh-CN"], record.City.Names["zh-CN"])
	return strings.TrimSpace(city)
}

// GetUserAgent 获取浏览器简称
func GetUserAgent(rawUserAgent string) string {
	if len(rawUserAgent) == 0 {
		return ""
	}
	ua := user_agent.New(rawUserAgent)
	name, version := ua.Browser()
	return fmt.Sprintf("%s %s", name, version)
}
