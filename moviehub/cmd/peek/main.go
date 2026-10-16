package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// peek อ่านเนื้อความ JWT เพื่อทดลองเท่านั้น ไม่ได้ตรวจสอบลายเซ็น
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "วิธีใช้: go run ./cmd/peek <TOKEN>")
		os.Exit(1)
	}
	parts := strings.Split(os.Args[1], ".")
	fmt.Println("ตั๋วมี", len(parts), "ท่อน")
	if len(parts) != 3 {
		fmt.Fprintln(os.Stderr, "JWT ต้องมี 3 ท่อน")
		os.Exit(1)
	}
	for i, name := range []string{"ส่วนหัว", "เนื้อความ"} {
		raw, err := base64.RawURLEncoding.DecodeString(parts[i])
		if err != nil {
			fmt.Fprintln(os.Stderr, name, "อ่านไม่ออก", err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", name, raw)
	}
	fmt.Println("ลายเซ็น", parts[2], "ยาว", len(parts[2]), "ตัว")
}
