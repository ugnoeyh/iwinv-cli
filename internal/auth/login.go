package auth

import (
	"fmt"
	"net/url"

	"github.com/playwright-community/playwright-go"
)

func IsLoginPage(page playwright.Page) (bool, error) {
	currentURL := page.URL()
	if currentURL != "" {
		parsed, err := url.Parse(currentURL)
		if err == nil {
			path := parsed.Path
			if path == "/member" || path == "/member/" || path == "/member/login" || path == "/member/login/" {
				return true, nil
			}
		}
	}

	raw, err := page.Evaluate(`() => {
		function isVisible(el) {
			if (!el) return false;
			const style = window.getComputedStyle(el);
			if (style.display === "none" || style.visibility === "hidden") return false;
			const rect = el.getBoundingClientRect();
			return rect.width > 0 && rect.height > 0;
		}

		const idInput = document.querySelector("input[name='id']");
		const pwInput = document.querySelector("input[name='pw']");
		return isVisible(idInput) && isVisible(pwInput);
	}`)
	if err != nil {
		return false, err
	}

	result, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("로그인 페이지 판별 결과를 해석할 수 없습니다")
	}

	return result, nil
}
