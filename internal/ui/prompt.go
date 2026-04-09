package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func PromptLine(label string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(label)
	value, err := reader.ReadString('\n')
	if err != nil && value == "" {
		fmt.Fprintf(os.Stderr, "⚠️ 입력 읽기 실패: %v\n", err)
	}
	return strings.TrimSpace(value)
}

func ConfirmAction(prompt string) bool {
	answer := strings.ToLower(PromptLine(prompt))
	return answer == "y" || answer == "yes"
}
