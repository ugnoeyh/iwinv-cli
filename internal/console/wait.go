package console

import (
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
)

const pollIntervalMs = 120

func waitForCreatePageReady(page playwright.Page, timeout time.Duration) error {
	if err := waitForCondition(page, timeout, func() (bool, error) {
		closeCreatePopup(page)
		return hasSelectableOptions(page, region1XPath, false)
	}); err != nil {
		return fmt.Errorf("리전 목록 준비 대기 실패: %w", err)
	}
	return nil
}

func waitForSelectableOptions(page playwright.Page, stepName, xpath string, isTable bool, timeout time.Duration) error {
	if err := waitForCondition(page, timeout, func() (bool, error) {
		return hasSelectableOptions(page, xpath, isTable)
	}); err != nil {
		return fmt.Errorf("[%s] 선택 항목 로딩 지연: %w", stepName, err)
	}
	return nil
}

func waitForXPathVisible(page playwright.Page, stepName, xpath string, timeout time.Duration) error {
	if err := waitForCondition(page, timeout, func() (bool, error) {
		return isXPathVisible(page, xpath)
	}); err != nil {
		return fmt.Errorf("[%s] 필수 요소 표시 대기 실패: %w", stepName, err)
	}
	return nil
}

func waitForTextMinLength(page playwright.Page, stepName, xpath string, minLen int, timeout time.Duration) error {
	if err := waitForCondition(page, timeout, func() (bool, error) {
		return hasMinLengthText(page, xpath, minLen)
	}); err != nil {
		return fmt.Errorf("[%s] 텍스트 로딩 대기 실패: %w", stepName, err)
	}
	return nil
}

func waitForCondition(page playwright.Page, timeout time.Duration, check func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for {
		ok, err := check()
		if err != nil {
			lastErr = err
		} else if ok {
			return nil
		}

		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return fmt.Errorf("timeout after %s", timeout)
		}

		page.WaitForTimeout(pollIntervalMs)
	}
}

func hasSelectableOptions(page playwright.Page, xpath string, isTable bool) (bool, error) {
	raw, err := page.Evaluate(`([xpath, isTable]) => {
		let container = document.evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (!container) return false;

		let items = isTable ? container.querySelectorAll('tr') : container.children;
		for (let el of items) {
			if (!el || el.offsetWidth === 0 || el.offsetHeight === 0) continue;

			let txt = el.innerText ? el.innerText.trim().replace(/\s+/g, ' ') : "";
			if (!txt || txt.includes("선택해주세요")) continue;

			let rb = el.querySelector ? el.querySelector('input[type="radio"]') : null;
			let unavailable = isTable && (txt.includes("품절") || (rb && rb.disabled));
			if (!unavailable) return true;
		}
		return false;
	}`, []interface{}{xpath, isTable})
	if err != nil {
		return false, err
	}

	ready, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("옵션 준비 상태를 해석할 수 없습니다")
	}

	return ready, nil
}

func isXPathVisible(page playwright.Page, xpath string) (bool, error) {
	raw, err := page.Evaluate(`(xpath) => {
		let el = document.evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (!el) return false;
		let style = window.getComputedStyle(el);
		if (style.display === 'none' || style.visibility === 'hidden') return false;
		let rect = el.getBoundingClientRect();
		return rect.width > 0 && rect.height > 0;
	}`, xpath)
	if err != nil {
		return false, err
	}

	visible, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("표시 상태를 해석할 수 없습니다")
	}

	return visible, nil
}

func hasMinLengthText(page playwright.Page, xpath string, minLen int) (bool, error) {
	raw, err := page.Evaluate(`([xpath, minLen]) => {
		let el = document.evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (!el) return false;
		let txt = el.innerText ? el.innerText.trim() : "";
		return txt.length >= minLen;
	}`, []interface{}{xpath, minLen})
	if err != nil {
		return false, err
	}

	ready, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("텍스트 길이 상태를 해석할 수 없습니다")
	}

	return ready, nil
}
