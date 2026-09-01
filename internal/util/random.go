package util

import "crypto/rand"

// RandomGraph 对应 RandomStringUtils.randomGraph，生成 n 个随机可打印 ASCII 字符（33-126）
func RandomGraph(n int) string {
	const (
		start = 33
		span  = 126 - 33 + 1
	)
	// 通过拒绝采样避免模偏差
	max := 256 / span * span
	buf := make([]byte, 0, n)
	tmp := make([]byte, 1)
	for len(buf) < n {
		_, _ = rand.Read(tmp)
		b := int(tmp[0])
		if b < max {
			buf = append(buf, byte(start+b%span))
		}
	}
	return string(buf)
}

// RandomAlphanumeric 对应 RandomStringUtils.randomAlphanumeric，生成 n 个随机字母数字字符
func RandomAlphanumeric(n int) string {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// 通过拒绝采样避免模偏差
	max := 256 / len(chars) * len(chars)
	buf := make([]byte, 0, n)
	tmp := make([]byte, 1)
	for len(buf) < n {
		_, _ = rand.Read(tmp)
		b := int(tmp[0])
		if b < max {
			buf = append(buf, chars[b%len(chars)])
		}
	}
	return string(buf)
}
