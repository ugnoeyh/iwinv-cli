package console

import (
	"fmt"
	"strings"
	"time"

	"iwinv-cli/internal/auth"

	"github.com/playwright-community/playwright-go"
)

func openCreateServicePage(page playwright.Page) error {
	if _, err := page.Goto(createServiceURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("서비스 생성 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "서비스 생성 페이지 접속"); err != nil {
		return err
	}

	if err := waitForCreatePageReady(page, 8*time.Second); err != nil {
		return fmt.Errorf("서비스 생성 페이지 준비 실패: %w", err)
	}

	return nil
}

func ensureAuthenticated(page playwright.Page, action string) error {
	onLogin, err := isLoginPage(page)
	if err != nil {
		return fmt.Errorf("%s 중 로그인 상태 확인 실패: %w", action, err)
	}
	if onLogin {
		return fmt.Errorf("%s 실패: 로그인 세션이 만료되었습니다. `--login`으로 재로그인 후 다시 시도하세요", action)
	}
	return nil
}

func isLoginPage(page playwright.Page) (bool, error) {
	return auth.IsLoginPage(page)
}

func closeCreatePopup(page playwright.Page) {
	popupBtn := page.Locator("xpath=" + popupCloseXPath)
	if count, _ := popupBtn.Count(); count > 0 {
		_ = popupBtn.Click(playwright.LocatorClickOptions{
			Force:   playwright.Bool(true),
			Timeout: playwright.Float(500),
		})
	}
}

func resolveOSTabXPath(osType string) string {
	switch strings.ToLower(strings.TrimSpace(osType)) {
	case "my", "my 운영체제":
		return osTabMyXPath
	case "솔루션", "solution":
		return osTabSolutionXPath
	default:
		return osTabDefaultXPath
	}
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		val = strings.TrimSpace(val)
		if val == "" {
			return 0
		}
		var n int
		_, _ = fmt.Sscanf(val, "%d", &n)
		return n
	default:
		return 0
	}
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func stringifyJSArray(raw interface{}) []string {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok && text != "" {
			result = append(result, text)
		}
	}
	return result
}
