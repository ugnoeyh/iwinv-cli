package output

import (
	"fmt"
	"strings"
)

func FormatCreateSummary(raw string) string {
	lines := strings.Split(raw, "\n")
	var cleaned []string
	skipFilters := []string{
		"확인", "견적서 인쇄", "(부가세 별도)", "- 회원님의",
		"2대 이상의", "- 반드시", "- 고객께서는", "스마일서브는", "데이터 망실에",
	}

	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}

		skip := false
		for _, filter := range skipFilters {
			if strings.Contains(text, filter) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		cleaned = append(cleaned, text)
	}

	var out string
	for _, text := range cleaned {
		if text == "ZONE" || text == "운영체제" || text == "하드웨어" || text == "이름 설정" || text == "수량" || text == "SSH Key & Script" || text == "비용" {
			out += fmt.Sprintf("\n▶ %s\n", text)
			continue
		}

		if text == "CPU" || text == "Memory" || text == "Disk" || text == "Traffic" || text == "Network" || text == "이름" || text == "SSH Key" || text == "Script" || text == "총 비용" {
			out += fmt.Sprintf("  - %-8s : ", text)
			continue
		}

		if strings.HasSuffix(out, " : ") {
			out += text + "\n"
		} else {
			out += "    " + text + "\n"
		}
	}

	return strings.TrimSpace(out)
}
