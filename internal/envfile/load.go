package envfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const PasswordKey = "IWINV_PW"

func LoadDefault() {
	Load(".env")
}

func Load(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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
	}
}
