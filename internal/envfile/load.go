package envfile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	PasswordKey            = "IWINV_PW"
	LoginIDKey             = "IWINV_ID"
	LegacyLoginPasswordKey = "IWINV_LOGIN_PW"
)

func LoadDefault() {
	if Load(".env") {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exeDir := filepath.Dir(exePath)
	_ = Load(filepath.Join(exeDir, ".env"))
}

func Load(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimPrefix(line, "\uFEFF")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimPrefix(line, "export ")
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)
		os.Setenv(key, val)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ .env 파일 읽기 중 오류: %v\n", err)
		return false
	}

	return true
}
