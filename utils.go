package main

import (
	"math/rand"
	"time"
)

const webPort = "8081"

type Config struct {
	IPFSNode string
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	rand.Seed(time.Now().UnixNano())

	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}

func StringRandom(length int) string {
	return StringWithCharset(length, charset)
}
