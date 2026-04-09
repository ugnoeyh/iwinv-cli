package console

import (
	"fmt"

	"github.com/playwright-community/playwright-go"
)

func RunFindSpecRegion(page playwright.Page, targetSpec string) error {
	fmt.Printf("\n🔍 '%s' 스펙을 제공하는 리전을 초고속 전수조사 중입니다...\n", targetSpec)

	if err := openCreateServicePage(page); err != nil {
		return err
	}

	script := `async ([targetSpec, r1X, r2X, specX]) => {
		const getNode = (xpath) => {
			return document.evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		};
		const delay = ms => new Promise(res => setTimeout(res, ms));
		const normalize = (text) => (text || "").replace(/\s+/g, "").toLowerCase().replace(/vcore/g, "vcpu");
		const matchesTarget = (specText, targetText) => {
			// "2 vCore" / "2 vCPU" 같은 순수 코어 검색은 숫자 경계를 지켜 32 vCPU 오탐을 방지한다.
			const pureCoreQuery = targetText.match(/^(\d+)vcpu$/);
			if (pureCoreQuery) {
				const n = pureCoreQuery[1];
				return new RegExp("(^|\\D)" + n + "vcpu").test(specText);
			}
			return specText.includes(targetText);
		};
		const isVisible = (el) => !!el && el.offsetWidth > 0 && el.offsetHeight > 0;
		const safeClick = (el) => {
			if (!el) return;
			el.scrollIntoView({ block: "center" });
			el.click();
			let child = el.querySelector("td") || el.firstElementChild;
			if (child) child.click();
		};
		const waitUntil = async (fn, timeoutMs = 2500, stepMs = 120) => {
			const end = Date.now() + timeoutMs;
			while (Date.now() < end) {
				try {
					if (fn()) return true;
				} catch (_) {}
				await delay(stepMs);
			}
			return false;
		};

		let r1Cont = getNode(r1X);
		if (!r1Cont) return { error: "리전 1차 요소를 찾을 수 없습니다." };

		let targetLower = normalize(targetSpec);
		let results = [];

		for (let i = 0; i < r1Cont.children.length; i++) {
			let r1 = r1Cont.children[i];
			if (!isVisible(r1)) continue;

			let r1Name = r1.innerText.trim().replace(/\s+/g, ' ');
			safeClick(r1);

			await waitUntil(() => {
				let cont = getNode(r2X);
				if (!cont) return false;
				for (let child of cont.children) {
					if (!isVisible(child)) continue;
					let txt = child.innerText ? child.innerText.trim() : "";
					if (txt && !txt.includes("선택해주세요")) return true;
				}
				return false;
			}, 3000);

			let r2Cont = getNode(r2X);
			if (!r2Cont) continue;

			for (let j = 0; j < r2Cont.children.length; j++) {
				let r2 = r2Cont.children[j];
				if (!isVisible(r2)) continue;

				let r2Name = r2.innerText.trim().replace(/\s+/g, ' ');
				safeClick(r2);

				await waitUntil(() => {
					let cont = getNode(specX);
					if (!cont) return false;
					let rows = cont.querySelectorAll("tr");
					for (let row of rows) {
						if (!isVisible(row)) continue;
						let rowText = row.innerText ? row.innerText.trim() : "";
						if (rowText) return true;
					}
					return false;
				}, 3500);

				let specCont = getNode(specX);
				if (!specCont) continue;

				let specs = specCont.querySelectorAll('tr');
				for (let k = 0; k < specs.length; k++) {
					let sp = specs[k];
					if (!isVisible(sp)) continue;

					let txt = normalize(sp.innerText);
					let rb = sp.querySelector('input[type="radio"]');
					let isSoldOut = txt.includes('품절') || (rb && rb.disabled);
					if (!isSoldOut && matchesTarget(txt, targetLower)) {
						results.push({
							region1: r1Name,
							region2: r2Name,
							spec: sp.innerText.trim().replace(/\s+/g, ' ')
						});
						break;
					}
				}
			}
		}

		return { results: results };
	}`

	rawRes, err := page.Evaluate(script, []interface{}{targetSpec, region1XPath, region2XPath, specXPath})
	if err != nil {
		return fmt.Errorf("초고속 스크립트 실행 오류: %w", err)
	}

	resMap, ok := rawRes.(map[string]interface{})
	if !ok {
		return fmt.Errorf("검색 결과 형식을 해석할 수 없습니다")
	}

	if resMap["error"] != nil {
		return fmt.Errorf("%v", resMap["error"])
	}

	resultsRaw, ok := resMap["results"].([]interface{})
	fmt.Println("==================================================")
	if !ok || len(resultsRaw) == 0 {
		fmt.Printf("❌ 전체 리전을 뒤졌지만 '%s' 스펙을 찾지 못했습니다.\n", targetSpec)
		return nil
	}

	fmt.Printf("🎉 총 %d개의 리전에서 생성 가능합니다.\n\n", len(resultsRaw))
	for _, value := range resultsRaw {
		item, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		fmt.Printf("  ✅ [리전 1차: %s] ➡️ [리전 2차: %s]\n     └ %s\n\n", item["region1"], item["region2"], item["spec"])
	}

	return nil
}

func RunCLIQuery(page playwright.Page, qR1 bool, qR2 string, qSpec bool, qOS bool, targetReg1, targetReg2, targetSpec, osType string) error {
	fmt.Println("🚀 데이터 조회를 위해 서비스 생성 페이지에 접속합니다...")

	if err := openCreateServicePage(page); err != nil {
		return err
	}

	if qR1 {
		fmt.Println("\n🔎 [리전 1차 목록]")
		printOptions(page, region1XPath, "region")
	}

	if qR2 != "" {
		fmt.Printf("\n🔎 ['%s'에 대한 리전 2차 목록]\n", qR2)
		if err := clickOptionByText(page, region1XPath, qR2, false); err != nil {
			return fmt.Errorf("리전 1차 '%s'를 찾지 못했습니다", qR2)
		}
		page.WaitForTimeout(1500)
		printOptions(page, region2XPath, "region")
	}

	if !qSpec && !qOS {
		return nil
	}

	if targetReg1 != "" {
		if err := clickOptionByText(page, region1XPath, targetReg1, false); err != nil {
			return fmt.Errorf("리전 1차 '%s'를 찾지 못했습니다", targetReg1)
		}
		page.WaitForTimeout(1500)
	}

	if targetReg2 != "" {
		if err := clickOptionByText(page, region2XPath, targetReg2, false); err != nil {
			return fmt.Errorf("리전 2차 '%s'를 찾지 못했습니다", targetReg2)
		}
		page.WaitForTimeout(2000)
	}

	if qSpec {
		fmt.Println("\n🔎 [서버 스펙 목록]")
		printOptions(page, specXPath, "spec")
	}

	if !qOS {
		return nil
	}

	if targetSpec != "" {
		if err := clickOptionByText(page, specXPath, targetSpec, true); err != nil {
			return fmt.Errorf("스펙 '%s'를 찾지 못했습니다", targetSpec)
		}
		page.WaitForTimeout(1500)
	}

	osTab := page.Locator("xpath=" + resolveOSTabXPath(osType))
	if count, _ := osTab.Count(); count > 0 {
		_ = osTab.Click()
	}
	page.WaitForTimeout(1000)

	fmt.Printf("\n🔎 ['%s' 탭 운영체제(OS) 목록]\n", osType)
	printOptions(page, osTableXPath, "os")
	return nil
}
