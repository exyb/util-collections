package qrcode

import (
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"os"
	"os/exec"

	"github.com/skip2/go-qrcode"
)

// 生成 OTP 密钥
func generateOTPKey(username string) string {

	const obfuscationString = "7A!14LTh)Ag7$G8O+/J!$*)w73|.F2n)"

	hash := sha1.Sum([]byte(username + obfuscationString))
	key := base32.StdEncoding.EncodeToString(hash[:])[:16]

	return key
}

func GeneratingQRCode(username string, saveQRCodeToFile bool) {
	// 生成 OTP 密钥
	otpKey := generateOTPKey(username)
	fmt.Println("Generated OTP key for user", username, ":", otpKey)

	// 生成 QR 代码
	otpURL := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=MPCSafeInput", username, otpKey)
	qrCode, err := qrcode.New(otpURL, qrcode.Medium)
	if err != nil {
		fmt.Println("Error creating QR code:", err)
		return
	}

	// 保存二维码为 PNG 文件
	pngFileName := username + "_otp_qr.png"
	err = qrCode.WriteFile(256, pngFileName)
	if err != nil {
		fmt.Println("Error saving PNG file:", err)
		return
	}

	// 使用 display 显示 PNG 文件（可选）
	cmd := exec.Command("bash", "-c", "cat "+pngFileName+"| display")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("Error running imgcat:", err)
		return
	}

	fmt.Println("QR code saved as", pngFileName)
	if !saveQRCodeToFile {
		os.Remove(pngFileName)
	}
}
