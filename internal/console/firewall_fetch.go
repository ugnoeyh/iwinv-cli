package console

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"strings"
)

func hasRuleUniqueInForm(page playwright.Page, tab, protocol, port, ruleIP string) (bool, error) {
	ruleIP = normalizeRuleIPInput(ruleIP)
	raw, err := page.Evaluate(`([tab, protocol, port, ruleIP]) => {
		const selector = "input[name='" + tab + "[unique][]']";
		const values = Array.from(document.querySelectorAll(selector))
			.map((el) => (el.value || "").trim())
			.filter(Boolean);

		const candidates = new Set();
		const ip = (ruleIP || "").trim();
		candidates.add(protocol + "," + port + "," + ip);
		if (ip && !ip.includes("/")) {
			candidates.add(protocol + "," + port + "," + ip + "/32");
		}
		if (ip && ip.endsWith("/32")) {
			candidates.add(protocol + "," + port + "," + ip.slice(0, -3));
		}
		if (ip === "0.0.0.0") {
			candidates.add(protocol + "," + port + ",0.0.0.0/0");
		}
		if (ip === "0.0.0.0/0") {
			candidates.add(protocol + "," + port + ",0.0.0.0");
		}

		for (const c of candidates) {
			if (values.includes(c)) return true;
		}
		return false;
	}`, []interface{}{tab, protocol, port, ruleIP})
	if err != nil {
		return false, err
	}

	ok, parsed := raw.(bool)
	if !parsed {
		return false, fmt.Errorf("룰 unique 검증 결과를 해석할 수 없습니다")
	}
	return ok, nil
}

func fetchFirewallTabRows(page playwright.Page, tab, idx string) ([][]string, error) {
	raw, err := page.Evaluate(`async ([apiBase, tab, idx]) => {
		const endpoint = apiBase + "/" + tab + "?idx=" + encodeURIComponent(idx) + "&ajax=true";
		const csrf = document.querySelector('meta[name="csrf-token"]')?.content || "";
		const headers = {
			"accept": "text/html, */*; q=0.01",
			"x-requested-with": "XMLHttpRequest"
		};
		if (csrf) headers["x-csrf-token"] = csrf;

		const res = await fetch(endpoint, {
			method: "GET",
			headers,
			credentials: "same-origin"
		});
		const body = await res.text();

		const doc = new DOMParser().parseFromString(body, "text/html");
		const rows = [];
		for (const tr of doc.querySelectorAll("tr")) {
			const cells = Array.from(tr.querySelectorAll("th,td"))
				.map((el) => (el.innerText || "").replace(/\s+/g, " ").trim())
				.filter(Boolean);
			if (cells.length === 0) continue;
			rows.push(cells);
		}

		if (rows.length === 0) {
			const text = (doc.body?.innerText || body || "").replace(/\s+/g, " ").trim();
			if (text) rows.push([text]);
		}

		return {
			status: res.status,
			rows,
			endpoint,
			bodyPreview: body.slice(0, 240)
		};
	}`, []interface{}{firewallTabURL, tab, idx})
	if err != nil {
		return nil, fmt.Errorf("방화벽 탭 조회 스크립트 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("방화벽 탭 조회 응답 형식을 해석할 수 없습니다")
	}

	status := toInt(res["status"])
	if status != 200 {
		preview, _ := res["bodyPreview"].(string)
		endpoint, _ := res["endpoint"].(string)
		return nil, fmt.Errorf("방화벽 탭 조회 실패 (status=%d, endpoint=%s, body=%q)", status, endpoint, strings.TrimSpace(preview))
	}

	return stringifyJSMatrix(res["rows"]), nil
}

func fetchFirewallInternationalTargets(page playwright.Page, idx string) ([]string, [][]string, error) {
	raw, err := page.Evaluate(`async ([apiBase, idx]) => {
		const endpoint = apiBase + "/international?idx=" + encodeURIComponent(idx) + "&ajax=true";
		const csrf = document.querySelector('meta[name="csrf-token"]')?.content || "";
		const headers = {
			"accept": "text/html, */*; q=0.01",
			"x-requested-with": "XMLHttpRequest"
		};
		if (csrf) headers["x-csrf-token"] = csrf;

		const res = await fetch(endpoint, {
			method: "GET",
			headers,
			credentials: "same-origin"
		});
		const body = await res.text();

		const doc = new DOMParser().parseFromString(body, "text/html");
		const normalize = (v) => String(v || "").replace(/\s+/g, " ").trim();
		const targets = [];
		const seen = new Set();
		const pushTarget = (v) => {
			const n = normalize(v).toUpperCase();
			if (!n || seen.has(n)) return;
			seen.add(n);
			targets.push(n);
		};

		for (const el of doc.querySelectorAll("[name='international[target][]']")) {
			const tag = (el.tagName || "").toLowerCase();
			if (tag === "select") {
				if (el.multiple) {
					for (const opt of Array.from(el.selectedOptions || [])) {
						pushTarget(opt?.value || opt?.textContent || "");
					}
				} else {
					const idx = Number(el.selectedIndex || 0);
					const opt = el.options && idx >= 0 ? el.options[idx] : null;
					pushTarget((opt && (opt.value || opt.textContent)) || el.value || "");
				}
			} else {
				pushTarget(el.value || el.getAttribute("value") || el.textContent || "");
			}
		}

		const rows = [];
		for (const tr of doc.querySelectorAll("tr")) {
			const cells = Array.from(tr.querySelectorAll("th,td"))
				.map((el) => normalize(el.innerText || ""))
				.filter(Boolean);
			if (cells.length === 0) continue;
			rows.push(cells);
		}

		return {
			status: res.status,
			endpoint,
			bodyPreview: body.slice(0, 240),
			targets,
			rows
		};
	}`, []interface{}{firewallTabURL, idx})
	if err != nil {
		return nil, nil, fmt.Errorf("국제망 탭 조회 스크립트 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("국제망 탭 조회 응답 형식을 해석할 수 없습니다")
	}

	status := toInt(res["status"])
	if status != 200 {
		preview, _ := res["bodyPreview"].(string)
		endpoint, _ := res["endpoint"].(string)
		return nil, nil, fmt.Errorf("국제망 탭 조회 실패 (status=%d, endpoint=%s, body=%q)", status, endpoint, strings.TrimSpace(preview))
	}

	rawTargets := stringifyJSArray(res["targets"])
	targets := normalizeFirewallInternationalTargetValues(rawTargets)
	rows := stringifyJSMatrix(res["rows"])
	if len(targets) == 0 && len(rows) > 0 {
		targets = inferInternationalTargetsFromRows(rows)
	}
	return targets, rows, nil
}

func fetchFirewallBotTargets(page playwright.Page, idx string) ([]string, [][]string, error) {
	raw, err := page.Evaluate(`async ([apiBase, idx]) => {
		const endpoint = apiBase + "/bot?idx=" + encodeURIComponent(idx) + "&ajax=true";
		const csrf = document.querySelector('meta[name="csrf-token"]')?.content || "";
		const headers = {
			"accept": "text/html, */*; q=0.01",
			"x-requested-with": "XMLHttpRequest"
		};
		if (csrf) headers["x-csrf-token"] = csrf;

		const res = await fetch(endpoint, {
			method: "GET",
			headers,
			credentials: "same-origin"
		});
		const body = await res.text();

		const doc = new DOMParser().parseFromString(body, "text/html");
		const normalize = (v) => String(v || "").replace(/\s+/g, " ").trim();
		const inferBotCode = (text) => {
			const t = String(text || "").replace(/\s+/g, " ").trim().toUpperCase();
			if (!t) return "";
			if (t === "ALL" || t.includes("전체")) return "ALL";
			if (t.includes("GOOGLE") || t.includes("구글")) return "GOOGLE";
			if (t.includes("NAVER") || t.includes("네이버")) return "NAVER";
			if (t.includes("DAUM") || t.includes("다음")) return "DAUM";
			return "";
		};
		const targets = [];
		const seen = new Set();
		const pushTarget = (v) => {
			const n = inferBotCode(v);
			if (!n || seen.has(n)) return;
			seen.add(n);
			targets.push(n);
		};

		const idxNodes = Array.from(doc.querySelectorAll("[name='bot[idx][]']"));
		const targetNodes = Array.from(doc.querySelectorAll("[name='bot[target][]']"));
		const pairLen = Math.max(idxNodes.length, targetNodes.length);
		for (let i = 0; i < pairLen; i++) {
			const idxNode = idxNodes[i] || null;
			const targetNode = targetNodes[i] || null;
			const idxValue = normalize((idxNode && idxNode.value) || "");
			let rawTarget = "";
			if (targetNode) {
				const tag = (targetNode.tagName || "").toLowerCase();
				if (tag === "select") {
					if (targetNode.multiple) {
						for (const opt of Array.from(targetNode.selectedOptions || [])) {
							pushTarget(opt?.value || opt?.textContent || "");
						}
						continue;
					}
					const selectedIdx = Number(targetNode.selectedIndex || 0);
					const opt = targetNode.options && selectedIdx >= 0 ? targetNode.options[selectedIdx] : null;
					rawTarget = (opt && (opt.value || opt.textContent)) || targetNode.value || "";
				} else {
					rawTarget = targetNode.value || targetNode.getAttribute("value") || targetNode.textContent || "";
				}
			}
			// 템플릿/기본 행(빈 idx)은 실제 적용 정책으로 간주하지 않는다.
			if (!idxValue) continue;
			pushTarget(rawTarget);
		}

		const rows = [];
		for (const tr of doc.querySelectorAll("tr")) {
			const cells = Array.from(tr.querySelectorAll("th,td"))
				.map((el) => normalize(el.innerText || ""))
				.filter(Boolean);
			if (cells.length === 0) continue;
			rows.push(cells);
		}
		const fullText = normalize((doc.body && (doc.body.innerText || doc.body.textContent)) || body);
		const noPolicyHint =
			fullText.includes("설정된 정책이 없습니다") ||
			fullText.includes("정책변경 설정된 정책이 없습니다") ||
			fullText.includes("등록된 정책이 없습니다");
		if (noPolicyHint && targets.length === 0 && rows.length === 0) {
			// clear 상태를 검증 단계에서 안정적으로 식별할 수 있도록 no-policy 힌트를 행으로 남긴다.
			rows.push(["설정된 정책이 없습니다"]);
		}

		return {
			status: res.status,
			endpoint,
			bodyPreview: body.slice(0, 240),
			idxNodeCount: idxNodes.length,
			targetNodeCount: targetNodes.length,
			noPolicyHint,
			targets,
			rows
		};
	}`, []interface{}{firewallTabURL, idx})
	if err != nil {
		return nil, nil, fmt.Errorf("검색봇 탭 조회 스크립트 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("검색봇 탭 조회 응답 형식을 해석할 수 없습니다")
	}

	status := toInt(res["status"])
	if status != 200 {
		preview, _ := res["bodyPreview"].(string)
		endpoint, _ := res["endpoint"].(string)
		return nil, nil, fmt.Errorf("검색봇 탭 조회 실패 (status=%d, endpoint=%s, body=%q)", status, endpoint, strings.TrimSpace(preview))
	}

	rawTargets := stringifyJSArray(res["targets"])
	targets := normalizeFirewallBotTargetValues(rawTargets)
	rows := stringifyJSMatrix(res["rows"])
	noPolicyHint, _ := res["noPolicyHint"].(bool)
	if len(targets) == 0 && !noPolicyHint {
		targets = normalizeFirewallBotTargetValues(inferFirewallBotTargetsFromRows(rows))
	}
	return targets, rows, nil
}
