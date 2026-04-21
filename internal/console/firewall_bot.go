package console

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"strings"
	"time"
)

func RunSetFirewallBot(page playwright.Page, policyRef, targetsInput string, debug bool) error {
	targets, err := normalizeFirewallBotTargets(targetsInput)
	if err != nil {
		return err
	}
	return runUpdateFirewallBot(page, policyRef, targets, debug, "설정")
}

func RunRemoveFirewallBot(page playwright.Page, policyRef, removeTargetsInput string, debug bool) error {
	removeTargets, err := normalizeFirewallBotRemoveTargets(removeTargetsInput)
	if err != nil {
		return err
	}
	return runRemoveFirewallTargets(
		page,
		policyRef,
		removeTargets,
		debug,
		"검색봇",
		"검색봇",
		"bot",
		fetchFirewallBotTargets,
		runUpdateFirewallBot,
	)
}

func RunClearFirewallBot(page playwright.Page, policyRef string, debug bool) error {
	return runUpdateFirewallBot(page, policyRef, []string{}, debug, "전체 제거")
}

func runUpdateFirewallBot(page playwright.Page, policyRef string, targets []string, debug bool, actionLabel string) error {
	policy, err := resolveFirewallPolicyReference(page, policyRef, debug, true)
	if err != nil {
		return err
	}

	targetText := formatFirewallTargetText(targets)
	fmt.Printf("🚀 ELCAP 검색봇 접근 정책을 %s 중입니다... (%s)\n", actionLabel, targetText)
	firewallDebugf(debug, "bot set start | action=%s ref=%s idx=%s targets=%v", actionLabel, policyRef, policy.Idx, targets)
	firewallDebugf(debug, "bot engine=edit-form-v1")

	editPageURL, err := openFirewallPolicyEditPage(page, policy.Idx, "bot")
	if err != nil {
		return err
	}
	firewallDebugf(debug, "goto edit page | url=%s", editPageURL)

	formCounts, countErr := getFirewallFormSectionCounts(page)
	if countErr != nil {
		return fmt.Errorf("검색봇 정책 저장 전 폼 상태 확인 실패: %w", countErr)
	}
	firewallDebugf(
		debug,
		"bot form snapshot title=%q inbound=%d outbound=%d international=%d bot=%d",
		formCounts.Title,
		formCounts.InboundCount,
		formCounts.OutboundCount,
		formCounts.InternationalCount,
		formCounts.BotCount,
	)
	if strings.TrimSpace(formCounts.Title) == "" && policy.TitleFallback == "" {
		return fmt.Errorf("검색봇 정책 저장 중단: 현재 폼에 정책명(title)이 비어 있고 대체 정책명도 찾지 못했습니다")
	}

	actualTargets, rows, precheckErr := fetchFirewallBotTargets(page, policy.Idx)
	if precheckErr == nil {
		if equalFirewallBotTargets(targets, actualTargets) {
			fmt.Printf("ℹ️ [%s | IDX:%s] 검색봇 접근 정책이 이미 동일하여 저장을 건너뜁니다: %s\n", strings.TrimSpace(policy.ResolvedName), policy.Idx, strings.Join(actualTargets, ","))
			firewallDebugf(debug, "bot precheck same targets | %v", actualTargets)
			return nil
		}
		firewallDebugf(debug, "bot precheck mismatch actual=%v expected=%v rows=%d", actualTargets, targets, len(rows))
	} else {
		firewallDebugf(debug, "bot precheck error | %v", precheckErr)
	}

	_, submitDiag, err := submitFirewallBotTargets(page, policy.Idx, targets, policy.TitleFallback)
	if err != nil {
		firewallDebugf(debug, "bot submit failed | %v", err)
		logFirewallSubmitDiagnostics(debug, submitDiag)
		return fmt.Errorf("검색봇 접근 정책 저장 실패: %w", err)
	}
	logFirewallSubmitDiagnostics(debug, submitDiag)

	ok, verifyErr := waitForFirewallBotApplied(page, policy.Idx, targets, 22*time.Second, debug)
	if ok {
		fmt.Printf("✅ [%s | IDX:%s] 검색봇 접근 정책 %s 완료: %s\n", strings.TrimSpace(policy.ResolvedName), policy.Idx, actionLabel, targetText)
		return nil
	}
	if verifyErr != nil {
		return fmt.Errorf("검색봇 접근 정책 저장 후 검증 실패: %w", verifyErr)
	}
	return fmt.Errorf("검색봇 접근 정책 저장 후 반영을 확인하지 못했습니다. --firewall-tab bot --firewall-ref \"%s\"로 재확인하세요", policy.Idx)
}

func submitFirewallBotTargets(page playwright.Page, idx string, targets []string, titleFallback string) (bool, map[string]interface{}, error) {
	raw, err := page.Evaluate(`async ([idx, targets, titleFallback]) => {
		const form =
			document.querySelector("form[name='modal']") ||
			document.querySelector("form[action='/firewall']") ||
			document.querySelector("form[action='https://console.iwinv.kr/firewall']") ||
			document.querySelector("form[action*='/firewall']") ||
			document.querySelector("main form");
		if (!form) {
			return { success: false, status: 0, reason: "저장 폼(form)을 찾지 못했습니다.", bodyHasUnique: false, diag: { formFound: false, currentURL: location.href } };
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
		};
		const appendPairToForm = (name, value) => {
			const el = document.createElement("input");
			el.type = "hidden";
			el.name = name;
			el.value = value;
			form.appendChild(el);
		};

		ensureHidden("_token", getValue("_token") || (document.querySelector('meta[name="csrf-token"]')?.content || ""));
		ensureHidden("idx", idx);
		ensureHidden("revision", getValue("revision") || "0");
		ensureHidden("firewallPolicyUnit", getValue("firewallPolicyUnit") || "100");
		ensureHidden("title", getValue("title") || String(titleFallback || "").trim() || "");
		ensureHidden("icmp", getValue("icmp") || "N");
		const existingType = getValue("_type");
		if (!existingType) {
			const fallbackTypeMap = {
				"idx": "hidden",
				"title": "hidden",
				"icmp": "radio",
				"inbound[title][]": "select-one",
				"inbound[protocol][]": "text",
				"inbound[port][]": "text",
				"inbound[ip][]": "text",
				"inbound[content][]": "text",
				"inbound[unique][]": "hidden",
				"outbound[title][]": "select-one",
				"outbound[protocol][]": "text",
				"outbound[port][]": "text",
				"outbound[ip][]": "text",
				"outbound[content][]": "text",
				"outbound[unique][]": "hidden",
				"international[target][]": "select-one",
				"bot[target][]": "select-one"
			};
			ensureHidden("_type", JSON.stringify(fallbackTypeMap));
		} else {
			ensureHidden("_type", existingType);
		}

		const required = ["_token", "idx", "revision", "firewallPolicyUnit", "_type"];
		const missing = required.filter((k) => !getValue(k));
		const diag = {
			formFound: true,
			formAction: form.getAttribute("action") || "",
			currentURL: location.href,
			requiredSnapshot: {
				_token: getValue("_token"),
				idx: getValue("idx"),
				revision: getValue("revision"),
				firewallPolicyUnit: getValue("firewallPolicyUnit"),
				_type_len: (getValue("_type") || "").length
			},
			botTargets: targets
		};
		if (missing.length > 0) {
			return { success: false, status: 0, reason: "필수 폼 필드가 누락되었습니다.", missing, bodyHasUnique: false, diag };
		}

		const inferBotCode = (text) => {
			const t = String(text || "").replace(/\s+/g, " ").trim().toUpperCase();
			if (!t) return "";
			if (t === "ALL" || t.includes("전체")) return "ALL";
			if (t.includes("GOOGLE") || t.includes("구글")) return "GOOGLE";
			if (t.includes("NAVER") || t.includes("네이버")) return "NAVER";
			if (t.includes("DAUM") || t.includes("다음")) return "DAUM";
			return "";
		};
		const extractNumericTokens = (text) => {
			const src = String(text || "");
			const out = [];
			const re = /(?:^|[^\d])(\d{3,12})(?=[^\d]|$)/g;
			let m;
			while ((m = re.exec(src)) !== null) {
				const num = String(m[1] || "").trim();
				if (!num) continue;
				out.push(num);
			}
			return out;
		};
		const pickCandidateIdx = (numbers, policyIdx) => {
			for (const n of numbers) {
				const num = String(n || "").trim();
				if (!num) continue;
				if (num === policyIdx) continue;
				return num;
			}
			return "";
		};

		const existingBotPairs = [];
		const existingIdxNodes = Array.from(document.querySelectorAll("[name='bot[idx][]']"));
		const existingTargetNodes = Array.from(document.querySelectorAll("[name='bot[target][]']"));
		const existingPairLen = Math.max(existingIdxNodes.length, existingTargetNodes.length);
		for (let i = 0; i < existingPairLen; i++) {
			const idxNode = existingIdxNodes[i] || null;
			const targetNode = existingTargetNodes[i] || null;
			const ownerForm = (targetNode && targetNode.form) || (idxNode && idxNode.form) || null;
			if (ownerForm && ownerForm !== form) continue;

			const targetValue = inferBotCode((targetNode && targetNode.value) || "");
			if (!targetValue) continue;
			const policyIdx = String(idx || "").trim();
			let idxValue = String((idxNode && idxNode.value) || "").trim();
			if (!idxValue) {
				idxValue = pickCandidateIdx(extractNumericTokens(targetNode ? (targetNode.value || targetNode.getAttribute("value") || targetNode.textContent || "") : ""), policyIdx);
			}
			existingBotPairs.push({ idx: idxValue, target: targetValue });
		}

		if (existingBotPairs.length === 0) {
			try {
				const endpoint = "https://console.iwinv.kr/firewall/tab/bot?idx=" + encodeURIComponent(idx) + "&ajax=true";
				const csrf = document.querySelector('meta[name="csrf-token"]')?.content || "";
				const headers = {
					"accept": "text/html, */*; q=0.01",
					"x-requested-with": "XMLHttpRequest"
				};
				if (csrf) headers["x-csrf-token"] = csrf;

				const tabRes = await fetch(endpoint, { method: "GET", headers, credentials: "same-origin" });
				const tabBody = await tabRes.text();
				const tabDoc = new DOMParser().parseFromString(tabBody, "text/html");
				diag.beforeBotFetchStatus = tabRes.status;
				diag.beforeBotFetchBodyPreview = tabBody.slice(0, 240);

				const detectedTargets = [];
				const detectedSet = new Set();
				const pushDetected = (v) => {
					const t = inferBotCode(v);
					if (!t || detectedSet.has(t)) return;
					detectedSet.add(t);
					detectedTargets.push(t);
				};

				const tabIdxNodes = Array.from(tabDoc.querySelectorAll("[name='bot[idx][]']"));
				const tabTargetNodes = Array.from(tabDoc.querySelectorAll("[name='bot[target][]']"));
				const tabPairLen = Math.max(tabIdxNodes.length, tabTargetNodes.length);
				for (let i = 0; i < tabPairLen; i++) {
					const idxNode = tabIdxNodes[i] || null;
					const targetNode = tabTargetNodes[i] || null;
					const idxValue = String((idxNode && idxNode.value) || "").trim();
					let rawTarget = "";
					if (targetNode) {
						const tag = String(targetNode.tagName || "").toLowerCase();
						if (tag === "select") {
							const selected = targetNode.options && targetNode.selectedIndex >= 0 ? targetNode.options[targetNode.selectedIndex] : null;
							rawTarget = String((selected && (selected.value || selected.textContent)) || targetNode.value || "").trim();
						} else {
							rawTarget = String(targetNode.value || targetNode.getAttribute("value") || targetNode.textContent || "").trim();
						}
					}
					const target = inferBotCode(rawTarget);
					pushDetected(rawTarget);
					if (!target) continue;
					existingBotPairs.push({ idx: idxValue, target });
				}

				if (detectedTargets.length === 0) {
					pushDetected(tabDoc.body ? tabDoc.body.innerText || tabDoc.body.textContent || "" : tabBody);
				}
				diag.beforeBotDetected = detectedTargets;
			} catch (e) {
				diag.beforeBotFetchError = String(e);
			}
		}

		diag.beforeBot = existingBotPairs.map((x) => String(x.target || "") + ":" + String(x.idx || ""));
		const detectedTargetsOnly = Array.isArray(diag.beforeBotDetected) ? diag.beforeBotDetected : [];
		if (existingBotPairs.length === 0 && detectedTargetsOnly.length > 0 && Array.isArray(targets) && targets.length > 0) {
			return {
				success: false,
				status: 0,
				reason: "기존 검색봇 정책의 idx를 찾지 못해 안전하게 덮어쓸 수 없습니다. (기존: " + detectedTargetsOnly.join(",") + ")",
				bodyHasUnique: false,
				diag
			};
		}

		const idxPoolByTarget = {};
		const genericIdxPool = [];
		const seenIdx = new Set();
		for (const pair of existingBotPairs) {
			if (!idxPoolByTarget[pair.target]) idxPoolByTarget[pair.target] = [];
			if (pair.idx) idxPoolByTarget[pair.target].push(pair.idx);
			if (pair.idx && !seenIdx.has(pair.idx)) {
				seenIdx.add(pair.idx);
				genericIdxPool.push(pair.idx);
			}
		}
		const usedIdx = new Set();
		const desiredPairs = [];
		for (const target of targets) {
			const normalizedTarget = String(target || "").trim().toUpperCase();
			const pool = idxPoolByTarget[normalizedTarget] || [];
			let reuseIdx = "";
			while (pool.length > 0 && !reuseIdx) {
				const candidate = String(pool.shift() || "").trim();
				if (!candidate || usedIdx.has(candidate)) continue;
				reuseIdx = candidate;
			}
			while (!reuseIdx && genericIdxPool.length > 0) {
				const candidate = String(genericIdxPool.shift() || "").trim();
				if (!candidate || usedIdx.has(candidate)) continue;
				reuseIdx = candidate;
			}
			if (reuseIdx) usedIdx.add(reuseIdx);
			desiredPairs.push({ idx: reuseIdx, target: normalizedTarget });
		}
		diag.afterBot = desiredPairs.map((x) => String(x.target || "") + ":" + String(x.idx || ""));

		const desiredTargets = desiredPairs.map((x) => String(x.target || "").trim().toUpperCase()).filter(Boolean);
		if (desiredTargets.includes("ALL") && desiredTargets.length > 1) {
			return {
				success: false,
				status: 0,
				reason: "ALL(전체 차단)과 개별 검색봇 차단은 동시에 설정할 수 없습니다.",
				bodyHasUnique: false,
				diag
			};
		}

		const countSectionInputs = (prefix) => {
			return Array.from(document.querySelectorAll("[name^='" + prefix.replace(/'/g, "\\'") + "[']"))
				.filter((el) => !el.form || el.form === form)
				.length;
		};
		const fetchSectionFields = async (tab, prefix) => {
			const endpoint = "https://console.iwinv.kr/firewall/tab/" + tab + "?idx=" + encodeURIComponent(idx) + "&ajax=true";
			const csrf = document.querySelector('meta[name="csrf-token"]')?.content || "";
			const headers = { "accept": "text/html, */*; q=0.01", "x-requested-with": "XMLHttpRequest" };
			if (csrf) headers["x-csrf-token"] = csrf;
			const res = await fetch(endpoint, { method: "GET", headers, credentials: "same-origin" });
			const body = await res.text();
			const doc = new DOMParser().parseFromString(body, "text/html");
			const fields = [];
			for (const el of doc.querySelectorAll("input,select,textarea")) {
				const name = String(el.getAttribute("name") || "").trim();
				if (!name) continue;
				const include = name.startsWith(prefix + "[") || ((prefix === "inbound" || prefix === "outbound") && name === "ip");
				if (!include) continue;
				const tag = String(el.tagName || "").toLowerCase();
				const type = String(el.getAttribute("type") || "").toLowerCase();
				if (tag === "select") {
					if (el.multiple) {
						for (const opt of Array.from(el.selectedOptions || [])) {
							fields.push([name, String((opt && (opt.value || opt.textContent)) || "").trim()]);
						}
					} else {
						const idx = Number(el.selectedIndex || 0);
						const opt = el.options && idx >= 0 ? el.options[idx] : null;
						fields.push([name, String((opt && (opt.value || opt.textContent)) || el.value || "").trim()]);
					}
					continue;
				}
				if ((type === "checkbox" || type === "radio") && !el.checked) continue;
				fields.push([name, String(el.value || el.getAttribute("value") || "").trim()]);
			}
			return { status: res.status, count: fields.length, fields, bodyPreview: body.slice(0, 240) };
		};
		const hydrateSectionIfMissing = async (tab, prefix) => {
			const existingCount = countSectionInputs(prefix);
			diag[prefix + "ExistingCount"] = existingCount;
			if (existingCount > 0) return;
			const fetched = await fetchSectionFields(tab, prefix);
			diag[prefix + "HydrateStatus"] = fetched.status;
			diag[prefix + "HydrateCount"] = fetched.count;
			diag[prefix + "HydrateBodyPreview"] = fetched.bodyPreview;
			if (fetched.status !== 200) throw new Error(prefix + " 탭 하이드레이션 실패(status=" + fetched.status + ")");
			if (!Array.isArray(fetched.fields) || fetched.fields.length === 0) {
				diag[prefix + "HydrateSkippedNoRows"] = true;
				return;
			}
			for (const pair of fetched.fields) {
				const key = String((pair && pair[0]) || "");
				const value = String((pair && pair[1]) || "");
				if (!key) continue;
				appendPairToForm(key, value);
			}
		};
		try {
			await hydrateSectionIfMissing("inbound", "inbound");
			await hydrateSectionIfMissing("outbound", "outbound");
			await hydrateSectionIfMissing("international", "international");
		} catch (e) {
			return { success: false, status: 0, reason: "기존 정책 스냅샷 하이드레이션 실패: " + String(e), bodyHasUnique: false, diag };
		}

		const ensureRuleUnique = (prefix) => {
			const getNodes = (name) => Array.from(document.querySelectorAll("[name='" + name.replace(/'/g, "\\'") + "']")).filter((el) => !el.form || el.form === form);
			const protocols = getNodes(prefix + "[protocol][]");
			const ports = getNodes(prefix + "[port][]");
			const ips = getNodes(prefix + "[ip][]");
			const uniques = getNodes(prefix + "[unique][]");
			const maxLen = Math.max(protocols.length, ports.length, ips.length, uniques.length);
			let filled = 0;
			for (let i = 0; i < maxLen; i++) {
				const protocol = String((protocols[i] && protocols[i].value) || "").trim().toUpperCase();
				const port = String((ports[i] && ports[i].value) || "").trim();
				const ip = String((ips[i] && ips[i].value) || "").trim();
				const uniqueValue = protocol && port && ip ? (protocol + "," + port + "," + ip) : "";
				let uniqueNode = uniques[i] || null;
				if (!uniqueNode) {
					uniqueNode = document.createElement("input");
					uniqueNode.type = "hidden";
					uniqueNode.name = prefix + "[unique][]";
					form.appendChild(uniqueNode);
					uniques.push(uniqueNode);
				}
				if (uniqueNode.value !== uniqueValue) uniqueNode.value = uniqueValue;
				if (uniqueValue) filled += 1;
			}
			return filled;
		};
		diag.inboundUniqueFilled = ensureRuleUnique("inbound");
		diag.outboundUniqueFilled = ensureRuleUnique("outbound");

		const fd = new FormData(form);
		const getScopedValues = (name) =>
			Array.from(document.querySelectorAll("[name='" + name.replace(/'/g, "\\'") + "']"))
				.filter((el) => !el.form || el.form === form)
				.map((el) => String(el.value || "").trim());
		const rowAt = (arr, i) => (Array.isArray(arr) && i >= 0 && i < arr.length ? String(arr[i] || "").trim() : "");
		const collectRuleRows = (prefix, defaultBound) => {
			const idxs = getScopedValues(prefix + "[idx][]");
			const bounds = getScopedValues(prefix + "[bound][]");
			const titles = getScopedValues(prefix + "[title][]");
			const protocols = getScopedValues(prefix + "[protocol][]");
			const ports = getScopedValues(prefix + "[port][]");
			const ips = getScopedValues(prefix + "[ip][]");
			const contents = getScopedValues(prefix + "[content][]");
			const maxLen = Math.max(idxs.length, bounds.length, titles.length, protocols.length, ports.length, ips.length, contents.length);
			const rows = [];
			for (let i = 0; i < maxLen; i++) {
				const idxValue = rowAt(idxs, i);
				const boundValue = (rowAt(bounds, i) || defaultBound).toUpperCase();
				const titleValue = rowAt(titles, i);
				const protocol = rowAt(protocols, i).toUpperCase();
				const port = rowAt(ports, i);
				const ip = rowAt(ips, i);
				const content = rowAt(contents, i);
				if (!idxValue && !protocol && !port && !ip && !titleValue && !content) continue;
				if (!protocol || !port || !ip) continue;
				rows.push({
					idx: idxValue,
					bound: boundValue,
					title: titleValue || (protocol + " 직접입력"),
					protocol,
					port,
					ip,
					content,
					unique: protocol + "," + port + "," + ip,
				});
			}
			return rows;
		};
		const collectInternationalRows = () => {
			const idxs = getScopedValues("international[idx][]");
			const targets = getScopedValues("international[target][]");
			const maxLen = Math.max(idxs.length, targets.length);
			const rows = [];
			for (let i = 0; i < maxLen; i++) {
				const idxValue = rowAt(idxs, i);
				const target = rowAt(targets, i).toUpperCase();
				if (!idxValue && !target) continue;
				if (!target) continue;
				rows.push({ idx: idxValue, target });
			}
			return rows;
		};

		const inboundRows = collectRuleRows("inbound", "IN");
		const outboundRows = collectRuleRows("outbound", "OUT");
		const internationalRows = collectInternationalRows();
		diag.inboundSubmitRows = inboundRows.length;
		diag.outboundSubmitRows = outboundRows.length;
		diag.internationalSubmitRows = internationalRows.length;
		diag.botSubmitRows = desiredPairs.length;

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

		for (const row of inboundRows) {
			params.append("inbound[idx][]", row.idx);
			params.append("inbound[bound][]", row.bound);
			params.append("inbound[title][]", row.title);
			params.append("inbound[protocol][]", row.protocol);
			params.append("inbound[port][]", row.port);
			params.append("ip", "direct");
			params.append("inbound[ip][]", row.ip);
			params.append("inbound[unique][]", row.unique);
			params.append("inbound[content][]", row.content);
		}
		for (const row of outboundRows) {
			params.append("outbound[idx][]", row.idx);
			params.append("outbound[bound][]", row.bound);
			params.append("outbound[title][]", row.title);
			params.append("outbound[protocol][]", row.protocol);
			params.append("outbound[port][]", row.port);
			params.append("ip", "direct");
			params.append("outbound[ip][]", row.ip);
			params.append("outbound[unique][]", row.unique);
			params.append("outbound[content][]", row.content);
		}
		for (const row of internationalRows) {
			params.append("international[idx][]", row.idx);
			params.append("international[target][]", row.target);
		}
		for (const pair of desiredPairs) {
			params.append("bot[idx][]", String(pair.idx || "").trim());
			params.append("bot[target][]", String(pair.target || "").trim().toUpperCase());
		}
		diag.submittedBot = desiredPairs.map((x) => String(x.target || "") + ":" + String(x.idx || ""));
		const paramEntries = Array.from(params.entries());
		diag.submitParamCount = paramEntries.length;
		diag.submitParamPreview = paramEntries.slice(0, 80).map(([k, v]) => String(k) + "=" + String(v || "").slice(0, 120));

		const routeValue = getValue("route");
		const actionValue = form.getAttribute("action") || "";
		let submitURL = "https://console.iwinv.kr/firewall";
		if (routeValue) {
			submitURL = new URL(routeValue, location.origin).toString();
		} else if (actionValue && !/\/firewall\/tab(?:\/|$|\?)/.test(actionValue)) {
			submitURL = new URL(actionValue, location.origin).toString();
		}
		diag.submitURL = submitURL;
		diag.formRoute = routeValue;
		diag.formAction = actionValue;

		const headers = {
			"accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"content-type": "application/x-www-form-urlencoded; charset=UTF-8"
		};

		const res = await fetch(submitURL, { method: "POST", headers, body: params.toString(), credentials: "same-origin" });
		const body = await res.text();
		const status = res.status;
		const bodyPreview = body.slice(0, 500);
		diag.status = status;
		diag.bodyPreview = bodyPreview;

		const lower = body.toLowerCase();
		const looksLikeLogin = (lower.includes("input[name='id']") || lower.includes("name=\"id\"")) && lower.includes("name=\"pw\"");
		const hasErrorKeyword = lower.includes("오류") || lower.includes("error") || lower.includes("exception") || lower.includes("일시적인 장애") || lower.includes("장애가 발생");
		const hasSuccessKeyword = body.includes("정상적으로 적용하였습니다.") || body.includes("정보 수정이 완료되었습니다.");
		const success = status >= 200 && status < 300 && !looksLikeLogin && !hasErrorKeyword && hasSuccessKeyword;
		return { success, status, bodyHasUnique: false, diag };
	}`, []interface{}{idx, targets, titleFallback})
	if err != nil {
		return false, nil, fmt.Errorf("검색봇 정책 저장 스크립트 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("검색봇 정책 저장 응답 형식을 해석할 수 없습니다")
	}

	success, _ := res["success"].(bool)
	diag, _ := res["diag"].(map[string]interface{})
	if !success {
		status := toInt(res["status"])
		reason, _ := res["reason"].(string)
		missing := stringifyJSArray(res["missing"])
		if reason != "" {
			if len(missing) > 0 {
				return false, diag, fmt.Errorf("%s (누락: %s)", reason, strings.Join(missing, ", "))
			}
			return false, diag, fmt.Errorf("%s", reason)
		}
		preview := ""
		if p, ok := diag["bodyPreview"].(string); ok {
			preview = p
		}
		return false, diag, fmt.Errorf("검색봇 정책 저장 실패 (status=%d, body=%q)", status, strings.TrimSpace(preview))
	}

	return true, diag, nil
}
