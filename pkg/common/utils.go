package common

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"math/rand"
	"time"
)

var (
	letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func CheckError(err error) bool {
	if err != nil {
		Warnf("Error: %v\n", err)
		return true
	}
	return false
}

func ComputeHash(str string) string {
	hasher := sha512.New()
	hasher.Write([]byte(str))
	return hex.EncodeToString(hasher.Sum(nil))
}

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func EncodeToBase64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func DecodeBase64(str string) (string, error) {
	if decoded, err := base64.StdEncoding.DecodeString(str); err != nil {
		return "", err
	} else {
		return string(decoded), nil
	}
}
