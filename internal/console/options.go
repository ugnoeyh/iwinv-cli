package console

import (
	"fmt"
	"strings"

	"iwinv-cli/internal/domain"

	"github.com/playwright-community/playwright-go"
)

func printOptions(page playwright.Page, xpath, mode string) {
	list := parseOptions(page, xpath, mode)
	if len(list) == 0 {
		fmt.Println("  ❌ 조회 가능한 항목이 없습니다.")
		return
	}

	for _, item := range list {
		fmt.Printf("  - %s\n", item.Text)
	}
}

func parseOptions(page playwright.Page, xpath, mode string) []domain.OptionItem {
	raw, err := page.Evaluate(`([xpath, mode]) => {
		let res = document.evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		let cont = res.singleNodeValue;
		if (!cont) return { error: "not found" };

		function isElementHidden(el) {
			if (!el || el === document || el === document.body) return false;
			if (el.offsetWidth === 0 || el.offsetHeight === 0) return true;
			let style = window.getComputedStyle(el);
			return style.display === 'none' || style.visibility === 'hidden';
		}

		let items = [];
		let children = (mode === 'spec' || mode === 'os') ? cont.querySelectorAll('tr') : cont.children;
		for (let i = 0; i < children.length; i++) {
			let el = children[i];
			if (isElementHidden(el)) continue;

			let txt = el.innerText ? el.innerText.trim().replace(/\s+/g, ' ') : "";
			if (mode === 'spec' || mode === 'os') {
				let rb = el.querySelector('input[type="radio"]');
				if (rb && rb.disabled) continue;
				if (txt.includes('품절') || txt === "") continue;
			}

			if (txt !== "" && !txt.includes("선택해주세요")) {
				items.push({ index: i, text: txt });
			}
		}

		return { items: items };
	}`, []interface{}{xpath, mode})
	if err != nil {
		fmt.Printf("⚠️ 옵션 조회 스크립트 실행 오류: %v\n", err)
		return nil
	}

	resMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}

	var list []domain.OptionItem
	if itemsRaw, ok := resMap["items"].([]interface{}); ok {
		for _, value := range itemsRaw {
			itemMap, ok := value.(map[string]interface{})
			if !ok {
				continue
			}

			text, _ := itemMap["text"].(string)
			list = append(list, domain.OptionItem{
				Index: toInt(itemMap["index"]),
				Text:  text,
			})
		}
	}

	return list
}

func clickOptionByText(page playwright.Page, xpath, targetText string, isTable bool) error {
	result, err := selectOption(page, xpath, targetText, isTable, false)
	if err != nil {
		return err
	}
	if result["success"] != true {
		return fmt.Errorf("옵션 클릭 실패")
	}
	return nil
}

func selectRequiredOption(page playwright.Page, stepName, xpath, target string, isTable bool) error {
	if target == "" {
		return nil
	}

	result, err := selectOption(page, xpath, target, isTable, true)
	if err != nil {
		return fmt.Errorf("[%s] 스크립트 실행 오류: %w", stepName, err)
	}

	if result["success"] != true {
		reason, _ := result["reason"].(string)
		available := stringifyJSArray(result["available"])
		if len(available) > 0 {
			return fmt.Errorf("[%s] 선택 실패: %s | 찾으려는 값: %q | 현재 항목: %s", stepName, reason, target, strings.Join(available, " | "))
		}
		return fmt.Errorf("[%s] 선택 실패: %s | 찾으려는 값: %q", stepName, reason, target)
	}

	return nil
}

func selectOption(page playwright.Page, xpath, target string, isTable, requireAvailable bool) (map[string]interface{}, error) {
	raw, err := page.Evaluate(`([xpath, name, isTable, requireAvailable]) => {
		let container = document.evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (!container) {
			return { success: false, reason: "XPath 컨테이너를 찾을 수 없습니다.", available: [] };
		}

		let items = isTable ? container.querySelectorAll('tr') : container.children;
		let available = [];
		let targetName = name.replace(/\s+/g, ' ').trim();
		let matchedButUnavailable = false;

		for (let el of items) {
			if (el.offsetWidth === 0 || el.offsetHeight === 0) continue;

			let elText = el.innerText ? el.innerText.replace(/\s+/g, ' ').trim() : "";
			if (elText) available.push(elText);

			let rb = el.querySelector('input[type="radio"]');
			let unavailable = isTable && (elText.includes('품절') || (rb && rb.disabled));

			if (!elText.includes(targetName)) continue;
			if (requireAvailable && unavailable) {
				matchedButUnavailable = true;
				continue;
			}

			el.scrollIntoView({ block: 'center' });
			el.click();
			let child = el.querySelector('td') || el.firstElementChild;
			if (child) child.click();
			return { success: true, available: available };
		}

		let reason = matchedButUnavailable
			? "매칭 항목이 있지만 현재 선택할 수 없습니다."
			: "매칭되는 텍스트가 없습니다.";
		return { success: false, reason: reason, available: available };
	}`, []interface{}{xpath, target, isTable, requireAvailable})
	if err != nil {
		return nil, err
	}

	result, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("선택 결과 형식을 해석할 수 없습니다")
	}

	return result, nil
}
