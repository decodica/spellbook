package identity

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const letterBytes = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandomString(size int, src rand.Source) string {
	sb := strings.Builder{}
	sb.Grow(size)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := size-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			sb.WriteByte(letterBytes[idx])
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return sb.String()
}

type TokenGenerator interface {
	GenerateToken() string
}

type DefaultTokenGenerator struct {
	Seed int64
}

func (tg DefaultTokenGenerator) GenerateToken() string {
	const l = 16
	now := time.Now().UTC().Unix()
	src := rand.NewSource(tg.Seed)
	rs := RandomString(l, src)
	s := fmt.Sprintf("%s%s%d", rs, tokenSeparator, now)
	hasher := sha1.New()
	hasher.Write([]byte(s))
	hash := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return hash
}

// generator used to create service account tokens
type ServiceAccountTokenGenerator struct {
	DefaultTokenGenerator
}

func (sa ServiceAccountTokenGenerator) GenerateToken() string {
	tkn := sa.DefaultTokenGenerator.GenerateToken()
	return fmt.Sprintf("%s%s", SAIdentifier, tkn)
}
