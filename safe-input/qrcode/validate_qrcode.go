package qrcode

import (
	"crypto/sha1"
	"encoding/base32"
	"fmt"

	"github.com/pquerna/otp/totp"
)

// 生成 OTP 密钥
func reGenerateOTPKey(username string) string {
	const obfuscationString = "7A!14LTh)Ag7$G8O+/J!$*)w73|.F2n)"

	hash := sha1.Sum([]byte(username + obfuscationString))
	key := base32.StdEncoding.EncodeToString(hash[:])[:16]

	return key
}

// 验证 OTP
func ValidateOTP(inputOTP string, username string) bool {
	// 验证 OTP
	if totp.Validate(inputOTP, reGenerateOTPKey(username)) {
		fmt.Println("OTP code is valid!")
		return true
	} else {
		fmt.Println("Invalid OTP code.")
	}
	return false
}
