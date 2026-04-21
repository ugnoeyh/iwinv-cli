package console

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"strings"
	"time"
)

func RunListFirewallPolicies(page playwright.Page) error {
	fmt.Println("🚀 ELCAP 방화벽 정책 목록을 조회 중입니다...")

	if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
		return err
	}

	if err := waitForXPathVisible(page, "ELCAP 정책 테이블", firewallTable2XPath, 8*time.Second); err != nil {
		return err
	}

	policies, err := getFirewallPolicies(page)
	if err != nil {
		return err
	}

	if len(policies) == 0 {
		fmt.Println("❌ 등록된 ELCAP 방화벽 정책이 없습니다.")
		return nil
	}

	fmt.Println("\n=== [ELCAP 방화벽 정책 목록] ===")
	for i, policy := range policies {
		name := strings.TrimSpace(policy.Name)
		if name == "" {
			name = "(정책명 확인 불가)"
		}
		fmt.Printf("[%d] %-30s | IDX: %s\n", i+1, name, policy.Idx)
	}
	fmt.Println("================================")
	return nil
}

func RunCreateFirewallPolicy(page playwright.Page, policyName string, debug bool) error {
	name := strings.TrimSpace(policyName)
	if name == "" {
		return fmt.Errorf("생성할 방화벽 정책 이름이 비어 있습니다")
	}

	fmt.Printf("🚀 ELCAP 방화벽 정책을 생성 중입니다... (%s)\n", name)

	if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
		return err
	}
	if err := waitForXPathVisible(page, "ELCAP 정책 테이블", firewallTable2XPath, 8*time.Second); err != nil {
		return err
	}

	beforePolicies, err := getFirewallPolicies(page)
	if err != nil {
		return err
	}
	beforeMatchIdxSet, beforeMatchCount := snapshotFirewallPolicyNameMatches(beforePolicies, name)
	firewallDebugf(debug, "firewall create precheck | name=%q matchCount=%d", name, beforeMatchCount)

	_, createDiag, err := submitFirewallPolicyCreate(page, name)
	if err != nil {
		logFirewallSubmitDiagnostics(debug, createDiag)
		return fmt.Errorf("ELCAP 방화벽 정책 생성 실패: %w", err)
	}
	logFirewallSubmitDiagnostics(debug, createDiag)

	createdIdx, verifyErr := waitForFirewallPolicyCreated(page, name, beforeMatchIdxSet, beforeMatchCount, 18*time.Second, debug)
	if verifyErr != nil {
		return fmt.Errorf("ELCAP 방화벽 정책 생성 후 검증 실패: %w", verifyErr)
	}

	fmt.Printf("✅ ELCAP 방화벽 정책 생성 완료: %s (IDX:%s)\n", name, createdIdx)
	return nil
}

func getFirewallPolicies(page playwright.Page) ([]FirewallPolicy, error) {
	raw, err := page.Evaluate(`(tableXPath) => {
		let table = document.evaluate(tableXPath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (!table) {
			return { error: "방화벽 정책 테이블(table[2])을 찾지 못했습니다.", items: [] };
		}

		let rows = table.querySelectorAll("tbody tr[data-idx], tr[data-idx]");
		let items = [];

		for (let row of rows) {
			if (!row || row.offsetWidth === 0 || row.offsetHeight === 0) continue;

			let idx = (row.getAttribute("data-idx") || "").trim();
			if (!idx) continue;

			let name = "";
			let cells = row.querySelectorAll("td");
			for (let cell of cells) {
				let txt = cell.innerText ? cell.innerText.replace(/\s+/g, " ").trim() : "";
				if (!txt) continue;
				if (txt.includes("수정") || txt.includes("삭제")) continue;
				name = txt;
				break;
			}

			let raw = row.innerText ? row.innerText.replace(/\s+/g, " ").trim() : "";
			if (!name) name = raw;
			items.push({ idx, name, raw });
		}

		return { items };
	}`, firewallTable2XPath)
	if err != nil {
		return nil, fmt.Errorf("방화벽 목록 추출 스크립트 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("방화벽 목록 응답 형식을 해석할 수 없습니다")
	}

	if errText, _ := res["error"].(string); strings.TrimSpace(errText) != "" {
		return nil, fmt.Errorf("%s", errText)
	}

	itemsRaw, ok := res["items"].([]interface{})
	if !ok {
		return nil, nil
	}

	policies := make([]FirewallPolicy, 0, len(itemsRaw))
	for _, value := range itemsRaw {
		item, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		idx, _ := item["idx"].(string)
		name, _ := item["name"].(string)
		rawText, _ := item["raw"].(string)

		if strings.TrimSpace(idx) == "" {
			continue
		}

		policies = append(policies, FirewallPolicy{
			Idx:  strings.TrimSpace(idx),
			Name: strings.TrimSpace(name),
			Raw:  strings.TrimSpace(rawText),
		})
	}

	return policies, nil
}

func snapshotFirewallPolicyNameMatches(policies []FirewallPolicy, name string) (map[string]bool, int) {
	matchSet := map[string]bool{}
	matchCount := 0
	target := strings.TrimSpace(name)
	for _, p := range policies {
		if strings.TrimSpace(p.Name) != target {
			continue
		}
		matchSet[p.Idx] = true
		matchCount++
	}
	return matchSet, matchCount
}

func waitForFirewallPolicyCreated(
	page playwright.Page,
	policyName string,
	beforeMatchIdxSet map[string]bool,
	beforeMatchCount int,
	timeout time.Duration,
	debug bool,
) (string, error) {
	deadline := time.Now().Add(timeout)
	target := strings.TrimSpace(policyName)
	var lastErr error
	attempt := 0

	for {
		attempt++
		if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			lastErr = err
			firewallDebugf(debug, "firewall create verify attempt=%d goto error=%v", attempt, err)
		} else if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속(생성 검증)"); err != nil {
			lastErr = err
			firewallDebugf(debug, "firewall create verify attempt=%d auth error=%v", attempt, err)
		} else if err := waitForXPathVisible(page, "ELCAP 정책 테이블", firewallTable2XPath, 8*time.Second); err != nil {
			lastErr = err
			firewallDebugf(debug, "firewall create verify attempt=%d table wait error=%v", attempt, err)
		} else {
			policies, err := getFirewallPolicies(page)
			if err != nil {
				lastErr = err
				firewallDebugf(debug, "firewall create verify attempt=%d policy list error=%v", attempt, err)
			} else {
				exactMatches := make([]FirewallPolicy, 0, 4)
				for _, p := range policies {
					if strings.TrimSpace(p.Name) == target {
						exactMatches = append(exactMatches, p)
					}
				}

				if len(exactMatches) > beforeMatchCount {
					for _, p := range exactMatches {
						if !beforeMatchIdxSet[p.Idx] {
							firewallDebugf(debug, "firewall create verify attempt=%d new idx=%s", attempt, p.Idx)
							return p.Idx, nil
						}
					}
				}

				if beforeMatchCount == 0 && len(exactMatches) > 0 {
					firewallDebugf(debug, "firewall create verify attempt=%d first match idx=%s", attempt, exactMatches[0].Idx)
					return exactMatches[0].Idx, nil
				}

				firewallDebugf(
					debug,
					"firewall create verify attempt=%d waiting exactMatches=%d before=%d",
					attempt,
					len(exactMatches),
					beforeMatchCount,
				)
			}
		}

		if time.Now().After(deadline) {
			break
		}
		page.WaitForTimeout(700)
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("정책 목록에서 새 정책 '%s'의 생성 반영을 확인하지 못했습니다. --firewall-list로 재확인하세요", target)
}

func submitFirewallPolicyCreate(page playwright.Page, policyName string) (bool, map[string]interface{}, error) {
	raw, err := page.Evaluate(`async ([policyName]) => {
		const title = String(policyName || "").trim();
		const form =
			document.querySelector("form[name='modal']") ||
			document.querySelector("form[action='/firewall']") ||
			document.querySelector("form[action='https://console.iwinv.kr/firewall']") ||
			document.querySelector("form[action*='/firewall']") ||
			document.querySelector("main form");
		if (!form) {
			return { success: false, status: 0, reason: "생성 폼(form)을 찾지 못했습니다.", diag: { formFound: false, currentURL: location.href } };
		}

		const getField = (name) => {
			const inForm = Array.from(form.elements || []).find((el) => el && el.name === name);
			if (inForm) return inForm;
			return document.querySelector("[name='" + name.replace(/'/g, "\\'") + "']");
		};
		const getValue = (name) => {
			const el = getField(name);
			if (!el) return "";
			return String(el.value || "").trim();
		};
		const ensureHidden = (name, fallback) => {
			let el = Array.from(form.elements || []).find((x) => x && x.name === name);
			if (!el) {
				el = document.createElement("input");
				el.type = "hidden";
				el.name = name;
				form.appendChild(el);
			}
			if (String(el.value || "").trim() === "") {
				el.value = fallback;
			}
			return el;
		};

		const token = getValue("_token") || (document.querySelector('meta[name="csrf-token"]')?.content || "");
		const typeFallback = JSON.stringify({ idx: "hidden", title: "text", icmp: "radio", content: "text" });

		ensureHidden("_token", token);
		ensureHidden("idx", "");
		ensureHidden("revision", "");
		ensureHidden("firewallPolicyUnit", getValue("firewallPolicyUnit") || "100");
		ensureHidden("title", title);
		ensureHidden("icmp", getValue("icmp") || "N");
		ensureHidden("content", getValue("content") || title);
		ensureHidden("_type", getValue("_type") || typeFallback);

		const titleField = getField("title");
		if (titleField) titleField.value = title;
		const idxField = getField("idx");
		if (idxField) idxField.value = "";
		const revisionField = getField("revision");
		if (revisionField) revisionField.value = "";

		const fd = new FormData(form);
		const params = new URLSearchParams();
		for (const [k, v] of fd.entries()) {
			const key = String(k || "");
			if (
				key === "ip" ||
				key.startsWith("inbound[") ||
				key.startsWith("outbound[") ||
				key.startsWith("international[") ||
				key.startsWith("bot[")
			) {
				continue;
			}
			params.append(key, String(v));
		}

		params.set("_token", token);
		params.set("idx", "");
		params.set("revision", "");
		params.set("firewallPolicyUnit", getValue("firewallPolicyUnit") || "100");
		params.set("title", title);
		params.set("icmp", getValue("icmp") || "N");
		params.set("content", getValue("content") || title);
		params.set("_type", getValue("_type") || typeFallback);

		const routeValue = getValue("route");
		const actionValue = form.getAttribute("action") || "";
		let submitURL = "https://console.iwinv.kr/firewall";
		if (routeValue) {
			submitURL = new URL(routeValue, location.origin).toString();
		} else if (actionValue && !/\/firewall\/tab(?:\/|$|\?)/.test(actionValue)) {
			submitURL = new URL(actionValue, location.origin).toString();
		}

		const diag = {
			formFound: true,
			formAction: actionValue,
			formRoute: routeValue,
			currentURL: location.href,
			submitURL,
			submitParamCount: Array.from(params.entries()).length,
			submitParamPreview: Array.from(params.entries()).slice(0, 50).map(([k, v]) => String(k) + "=" + String(v || "").slice(0, 120))
		};

		if (!title) {
			return { success: false, status: 0, reason: "정책명이 비어 있습니다.", diag };
		}
		if (!token) {
			return { success: false, status: 0, reason: "_token이 비어 있습니다.", diag };
		}

		const headers = {
			"accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"content-type": "application/x-www-form-urlencoded; charset=UTF-8"
		};
		const res = await fetch(submitURL, {
			method: "POST",
			headers,
			body: params.toString(),
			credentials: "same-origin"
		});

		const body = await res.text();
		const status = res.status;
		const bodyPreview = body.slice(0, 500);
		diag.status = status;
		diag.bodyPreview = bodyPreview;

		const lower = body.toLowerCase();
		const looksLikeLogin = (lower.includes("input[name='id']") || lower.includes("name=\"id\"")) && lower.includes("name=\"pw\"");
		const hasErrorKeyword = lower.includes("오류") || lower.includes("error") || lower.includes("exception") || lower.includes("일시적인 장애") || lower.includes("장애가 발생");
		const hasSuccessKeyword = body.includes("정상적으로 적용하였습니다.") || body.includes("정보 수정이 완료되었습니다.");
		diag.looksLikeLogin = looksLikeLogin;
		diag.hasErrorKeyword = hasErrorKeyword;
		diag.hasSuccessKeyword = hasSuccessKeyword;

		const success = status >= 200 && status < 300 && !looksLikeLogin && !hasErrorKeyword;
		return { success, status, diag };
	}`, []interface{}{policyName})
	if err != nil {
		return false, nil, fmt.Errorf("방화벽 정책 생성 스크립트 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("방화벽 정책 생성 응답 형식을 해석할 수 없습니다")
	}

	success, _ := res["success"].(bool)
	diag, _ := res["diag"].(map[string]interface{})
	if !success {
		status := toInt(res["status"])
		reason, _ := res["reason"].(string)
		if reason != "" {
			return false, diag, fmt.Errorf("%s", reason)
		}
		preview := ""
		if p, ok := diag["bodyPreview"].(string); ok {
			preview = p
		}
		return false, diag, fmt.Errorf("방화벽 정책 생성 실패 (status=%d, body=%q)", status, strings.TrimSpace(preview))
	}

	return true, diag, nil
}
