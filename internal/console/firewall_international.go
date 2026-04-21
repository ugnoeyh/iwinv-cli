package console

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"strings"
	"time"
)

func RunSetFirewallInternational(page playwright.Page, policyRef, targetsInput string, debug bool) error {
	targets, err := normalizeFirewallInternationalTargets(targetsInput)
	if err != nil {
		return err
	}
	return runUpdateFirewallInternational(page, policyRef, targets, debug, "설정")
}

func RunRemoveFirewallInternational(page playwright.Page, policyRef, removeTargetsInput string, debug bool) error {
	removeTargets, err := normalizeFirewallInternationalRemoveTargets(removeTargetsInput)
	if err != nil {
		return err
	}
	return runRemoveFirewallTargets(
		page,
		policyRef,
		removeTargets,
		debug,
		"국제망 통신",
		"국제망",
		"international",
		fetchFirewallInternationalTargets,
		runUpdateFirewallInternational,
	)
}

func RunClearFirewallInternational(page playwright.Page, policyRef string, debug bool) error {
	return runUpdateFirewallInternational(page, policyRef, []string{}, debug, "전체 제거")
}

func runUpdateFirewallInternational(page playwright.Page, policyRef string, targets []string, debug bool, actionLabel string) error {
	policy, err := resolveFirewallPolicyReference(page, policyRef, debug, true)
	if err != nil {
		return err
	}

	targetText := formatFirewallTargetText(targets)
	fmt.Printf("🚀 ELCAP 국제망 통신 정책을 %s 중입니다... (%s)\n", actionLabel, targetText)
	firewallDebugf(debug, "international set start | action=%s ref=%s idx=%s targets=%v", actionLabel, policyRef, policy.Idx, targets)
	firewallDebugf(debug, "international engine=edit-form-v4")

	editPageURL, err := openFirewallPolicyEditPage(page, policy.Idx, "international")
	if err != nil {
		return err
	}
	firewallDebugf(debug, "goto edit page | url=%s", editPageURL)

	formCounts, countErr := getFirewallFormSectionCounts(page)
	if countErr != nil {
		return fmt.Errorf("국제망 저장 전 폼 상태 확인 실패: %w", countErr)
	}
	firewallDebugf(
		debug,
		"international form snapshot title=%q inbound=%d outbound=%d international=%d bot=%d",
		formCounts.Title,
		formCounts.InboundCount,
		formCounts.OutboundCount,
		formCounts.InternationalCount,
		formCounts.BotCount,
	)

	inRows, inErr := fetchFirewallTabRows(page, "inbound", policy.Idx)
	outRows, outErr := fetchFirewallTabRows(page, "outbound", policy.Idx)
	if inErr == nil && outErr == nil {
		inData := estimateFirewallRuleDataRows(inRows)
		outData := estimateFirewallRuleDataRows(outRows)
		if strings.TrimSpace(formCounts.Title) == "" {
			if policy.TitleFallback == "" {
				return fmt.Errorf("국제망 정책 저장 중단: 현재 폼에 정책명(title)이 비어 있고 대체 정책명도 찾지 못했습니다")
			}
			firewallDebugf(debug, "international form title empty; fallback title=%q will be used", policy.TitleFallback)
		}
		if inData > 0 && formCounts.InboundCount == 0 {
			firewallDebugf(debug, "international form inbound snapshot is empty; submit 단계에서 탭 데이터를 하이드레이션 시도합니다")
		}
		if outData > 0 && formCounts.OutboundCount == 0 {
			firewallDebugf(debug, "international form outbound snapshot is empty; submit 단계에서 탭 데이터를 하이드레이션 시도합니다")
		}
	} else {
		firewallDebugf(debug, "international safety precheck tab fetch warning | inboundErr=%v outboundErr=%v", inErr, outErr)
	}

	actualTargets, rows, precheckErr := fetchFirewallInternationalTargets(page, policy.Idx)
	if precheckErr == nil {
		if equalInternationalTargets(targets, actualTargets) {
			fmt.Printf("ℹ️ [%s | IDX:%s] 국제망 통신 정책이 이미 동일하여 저장을 건너뜁니다: %s\n", strings.TrimSpace(policy.ResolvedName), policy.Idx, strings.Join(actualTargets, ","))
			firewallDebugf(debug, "international precheck same targets | %v", actualTargets)
			return nil
		}
		firewallDebugf(debug, "international precheck mismatch actual=%v expected=%v rows=%d", actualTargets, targets, len(rows))
	} else {
		firewallDebugf(debug, "international precheck error | %v", precheckErr)
	}

	_, submitDiag, err := submitFirewallInternationalTargets(page, policy.Idx, targets, policy.TitleFallback)
	if err != nil && len(targets) > 0 && strings.Contains(err.Error(), "기존 국제망 정책의 idx를 찾지 못해 안전하게 덮어쓸 수 없습니다") {
		firewallDebugf(debug, "international submit fallback(two-phase) | reason=%v", err)
		logFirewallSubmitDiagnostics(debug, submitDiag)

		_, clearDiag, clearErr := submitFirewallInternationalTargets(page, policy.Idx, []string{}, policy.TitleFallback)
		if clearErr != nil {
			firewallDebugf(debug, "international clear phase failed | %v", clearErr)
			logFirewallSubmitDiagnostics(debug, clearDiag)
			return fmt.Errorf("국제망 통신 정책 저장 실패(1차 clear 단계): %w", clearErr)
		}
		logFirewallSubmitDiagnostics(debug, clearDiag)
		page.WaitForTimeout(500)

		_, submitDiag, err = submitFirewallInternationalTargets(page, policy.Idx, targets, policy.TitleFallback)
	}
	if err != nil {
		firewallDebugf(debug, "international submit failed | %v", err)
		logFirewallSubmitDiagnostics(debug, submitDiag)
		return fmt.Errorf("국제망 통신 정책 저장 실패: %w", err)
	}
	logFirewallSubmitDiagnostics(debug, submitDiag)

	ok, verifyErr := waitForFirewallInternationalApplied(page, policy.Idx, targets, 22*time.Second, debug)
	if ok {
		fmt.Printf("✅ [%s | IDX:%s] 국제망 통신 정책 %s 완료: %s\n", strings.TrimSpace(policy.ResolvedName), policy.Idx, actionLabel, targetText)
		return nil
	}
	if verifyErr != nil {
		return fmt.Errorf("국제망 통신 정책 저장 후 검증 실패: %w", verifyErr)
	}
	return fmt.Errorf("국제망 통신 정책 저장 후 반영을 확인하지 못했습니다. --firewall-tab international --firewall-ref \"%s\"로 재확인하세요", policy.Idx)
}

func submitFirewallInternationalTargets(page playwright.Page, idx string, targets []string, titleFallback string) (bool, map[string]interface{}, error) {
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
		const appendHidden = (name, value) => {
			const el = document.createElement("input");
			el.type = "hidden";
			el.name = name;
			el.value = value;
			form.appendChild(el);
		};
		const collectValues = (name) => {
			return Array.from(document.querySelectorAll("[name='" + name.replace(/'/g, "\\'") + "']"))
				.map((el) => String(el.value || "").trim())
				.filter(Boolean);
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
			targets
		};
		if (missing.length > 0) {
			return {
				success: false,
				status: 0,
				reason: "필수 폼 필드가 누락되었습니다.",
				missing,
				bodyHasUnique: false,
				diag
			};
		}

		const existingInternationalPairs = [];
		const existingIdxNodes = Array.from(document.querySelectorAll("[name='international[idx][]']"));
		const existingTargetNodes = Array.from(document.querySelectorAll("[name='international[target][]']"));
		const existingPairLen = Math.max(existingIdxNodes.length, existingTargetNodes.length);
		const inferTargetCode = (text) => {
			const t = String(text || "").replace(/\s+/g, " ").trim().toUpperCase();
			if (!t) return "";
			if (t.includes("FOREIGN") || t.includes("한국을 제외한 모든 국가") || t.includes("한국제외") || t.includes("한국만 허용") || t.includes("국내만 허용")) return "FOREIGN";
			if (t.includes("TAIWAN") || t.includes("대만")) return "TAIWAN";
			if (t.includes("CHINA") || t.includes("중국")) return "CHINA";
			if (t.includes("PHILIPPINES") || t.includes("PHILIPPINE") || t.includes("필리핀")) return "PHILIPPINES";
			if (t.includes("USA") || t.includes(" US ") || t.includes("미국")) return "USA";
			if (t.includes("JAPAN") || t.includes("일본")) return "JAPAN";
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

		for (let i = 0; i < existingPairLen; i++) {
			const idxNode = existingIdxNodes[i] || null;
			const targetNode = existingTargetNodes[i] || null;
			const ownerForm = (targetNode && targetNode.form) || (idxNode && idxNode.form) || null;
			if (ownerForm && ownerForm !== form) continue;

			const targetValue = String((targetNode && targetNode.value) || "").trim().toUpperCase();
			if (!targetValue) continue;
			const policyIdx = String(idx || "").trim();
			let idxValue = String((idxNode && idxNode.value) || "").trim();
			if (!idxValue) {
				idxValue = pickCandidateIdx(extractNumericTokens(targetNode ? (targetNode.value || targetNode.getAttribute("value") || targetNode.textContent || "") : ""), policyIdx);
			}
			if (!idxValue) continue;
			existingInternationalPairs.push({ idx: idxValue, target: targetValue });
		}

		// 국제망 값이 폼에 없을 때, 국제망 탭 AJAX 결과에서 기존 idx/target을 보완 수집한다.
		if (existingInternationalPairs.length === 0) {
			try {
				const endpoint = "https://console.iwinv.kr/firewall/tab/international?idx=" + encodeURIComponent(idx) + "&ajax=true";
				const csrf = document.querySelector('meta[name="csrf-token"]')?.content || "";
				const headers = {
					"accept": "text/html, */*; q=0.01",
					"x-requested-with": "XMLHttpRequest"
				};
				if (csrf) headers["x-csrf-token"] = csrf;

				const tabRes = await fetch(endpoint, {
					method: "GET",
					headers,
					credentials: "same-origin"
				});
				const tabBody = await tabRes.text();
				const tabDoc = new DOMParser().parseFromString(tabBody, "text/html");
				diag.beforeInternationalFetchStatus = tabRes.status;
				diag.beforeInternationalFetchBodyPreview = tabBody.slice(0, 240);
				const detectedTargets = [];
				const detectedSet = new Set();
				const pushDetectedTarget = (v) => {
					const t = inferTargetCode(v);
					if (!t || detectedSet.has(t)) return;
					detectedSet.add(t);
					detectedTargets.push(t);
				};

				// 1) hidden/select 필드에서 idx/target 페어를 우선 파싱
				const tabIdxNodes = Array.from(tabDoc.querySelectorAll("[name='international[idx][]']"));
				const tabTargetNodes = Array.from(tabDoc.querySelectorAll("[name='international[target][]']"));
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
					const target = inferTargetCode(rawTarget);
					pushDetectedTarget(rawTarget);
					if (!idxValue || !target) continue;
					existingInternationalPairs.push({ idx: idxValue, target });
				}

				// 2) row 단위에서 idx/target 보완 파싱 (data-idx, onclick/href 속성, outerHTML 숫자 토큰)
				const pairKeySet = new Set(existingInternationalPairs.map((x) => String(x.target || "") + ":" + String(x.idx || "")));
				const policyIdx = String(idx || "").trim();
				const pushPair = (rowIdx, targetText) => {
					const resolvedTarget = inferTargetCode(targetText);
					const idxValue = String(rowIdx || "").trim();
					if (!resolvedTarget || !idxValue) return;
					if (idxValue === policyIdx) return;
					const key = resolvedTarget + ":" + idxValue;
					if (pairKeySet.has(key)) return;
					pairKeySet.add(key);
					existingInternationalPairs.push({ idx: idxValue, target: resolvedTarget });
				};
				const filterPolicyIdx = (numbers) => numbers.filter((n) => String(n || "").trim() !== policyIdx);

				for (const row of tabDoc.querySelectorAll("tr,li")) {
					const rowText = String(row.innerText || row.textContent || "").trim();
					const target = inferTargetCode(rowText);
					if (!target) continue;
					pushDetectedTarget(rowText);

					let pickedIdx = String(row.getAttribute("data-idx") || "").trim();
					if (pickedIdx === policyIdx) pickedIdx = "";

					if (!pickedIdx) {
						const candidateNumbers = [];
						const pushAttrNumbers = (el) => {
							if (!el || !el.attributes) return;
							for (const attr of Array.from(el.attributes)) {
								const name = String(attr.name || "").toLowerCase();
								const value = String(attr.value || "");
								if (!value) continue;
								if (
									name.includes("idx") ||
									name === "href" ||
									name.startsWith("on") ||
									name === "value" ||
									name === "data-value" ||
									name === "id"
								) {
									candidateNumbers.push(...extractNumericTokens(value));
								}
							}
						};

						pushAttrNumbers(row);
						for (const child of row.querySelectorAll("*")) {
							pushAttrNumbers(child);
						}
						if (candidateNumbers.length === 0) {
							candidateNumbers.push(...extractNumericTokens(row.outerHTML || ""));
						}
						pickedIdx = pickCandidateIdx(filterPolicyIdx(candidateNumbers), policyIdx);
					}

					if (pickedIdx) {
						pushPair(pickedIdx, target);
					}
				}

				if (detectedTargets.length === 0) {
					pushDetectedTarget(tabDoc.body ? tabDoc.body.innerText || tabDoc.body.textContent || "" : tabBody);
				}
				diag.beforeInternationalDetected = detectedTargets;
			} catch (e) {
				diag.beforeInternationalFetchError = String(e);
			}
		}

		diag.beforeInternational = existingInternationalPairs.map((x) => String(x.target || "") + ":" + String(x.idx || ""));
		const detectedTargetsOnly = Array.isArray(diag.beforeInternationalDetected) ? diag.beforeInternationalDetected : [];
		if (existingInternationalPairs.length === 0 && detectedTargetsOnly.length > 0 && Array.isArray(targets) && targets.length > 0) {
			return {
				success: false,
				status: 0,
				reason: "기존 국제망 정책의 idx를 찾지 못해 안전하게 덮어쓸 수 없습니다. (기존: " + detectedTargetsOnly.join(",") + ")",
				bodyHasUnique: false,
				diag
			};
		}

		const idxPoolByTarget = {};
		const genericIdxPool = [];
		const seenIdx = new Set();
		for (const pair of existingInternationalPairs) {
			if (!pair.idx) continue;
			if (!idxPoolByTarget[pair.target]) idxPoolByTarget[pair.target] = [];
			idxPoolByTarget[pair.target].push(pair.idx);
			if (!seenIdx.has(pair.idx)) {
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

		diag.afterInternational = desiredPairs.map((x) => String(x.target || "") + ":" + String(x.idx || ""));
		const desiredTargets = desiredPairs.map((x) => String(x.target || "").trim().toUpperCase()).filter(Boolean);
		if (desiredTargets.includes("FOREIGN") && desiredTargets.length > 1) {
			return {
				success: false,
				status: 0,
				reason: "FOREIGN(한국만 허용)과 개별 국가 차단은 동시에 설정할 수 없습니다.",
				bodyHasUnique: false,
				diag
			};
		}

		const countSectionInputs = (prefix) => {
			return Array.from(document.querySelectorAll("[name^='" + prefix.replace(/'/g, "\\'") + "[']"))
				.filter((el) => !el.form || el.form === form)
				.length;
		};
		const appendPairToForm = (name, value) => {
			const el = document.createElement("input");
			el.type = "hidden";
			el.name = name;
			el.value = value;
			form.appendChild(el);
		};
		const fetchSectionFields = async (tab, prefix) => {
			const endpoint = "https://console.iwinv.kr/firewall/tab/" + tab + "?idx=" + encodeURIComponent(idx) + "&ajax=true";
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
			const fields = [];
			const seenRuleKeys = new Set();
			const collectNamedPairs = (root) => {
				const out = [];
				for (const el of (root || document).querySelectorAll("input,select,textarea")) {
					const name = String(el.getAttribute("name") || "").trim();
					if (!name) continue;

					const include =
						name.startsWith(prefix + "[") ||
						((prefix === "inbound" || prefix === "outbound") && name === "ip");
					if (!include) continue;

					const tag = String(el.tagName || "").toLowerCase();
					const type = String(el.getAttribute("type") || "").toLowerCase();

					if (tag === "select") {
						if (el.multiple) {
							for (const opt of Array.from(el.selectedOptions || [])) {
								out.push([name, String((opt && (opt.value || opt.textContent)) || "").trim()]);
							}
						} else {
							const idx = Number(el.selectedIndex || 0);
							const opt = el.options && idx >= 0 ? el.options[idx] : null;
							out.push([name, String((opt && (opt.value || opt.textContent)) || el.value || "").trim()]);
						}
						continue;
					}
					if (type === "checkbox" || type === "radio") {
						if (!el.checked) continue;
					}
					out.push([name, String(el.value || el.getAttribute("value") || "").trim()]);
				}
				return out;
			};
			const executeInlineScripts = (root) => {
				let executed = 0;
				for (const script of (root || document).querySelectorAll("script")) {
					const src = String(script.getAttribute("src") || "").trim();
					if (src) continue;
					const code = String(script.textContent || "").trim();
					if (!code) continue;
					try {
						(0, eval)(code);
						executed += 1;
					} catch (e) {
						// continue
					}
				}
				return executed;
			};

			let rawPairs = collectNamedPairs(doc);
			let missingIdxCount = 0;
			let hydratedByPanel = false;
			let panelHydrateCount = 0;
			let panelScriptsExecuted = 0;
			if (rawPairs.length === 0) {
				const panel = document.querySelector("#tab-" + tab) || document.querySelector("#tab-" + prefix);
				if (panel) {
					panel.innerHTML = body;
					panelScriptsExecuted = executeInlineScripts(panel);
					await new Promise((resolve) => setTimeout(resolve, 0));
					panelHydrateCount = countSectionInputs(prefix);
					if (panelHydrateCount > 0) {
						hydratedByPanel = true;
					} else {
						rawPairs = collectNamedPairs(panel);
					}
				}
			}
			if (hydratedByPanel) {
				return {
					status: res.status,
					count: panelHydrateCount,
					ruleCount: 0,
					missingIdxCount: 0,
					fields: [],
					dataRows: panelHydrateCount,
					bodyPreview: body.slice(0, 240),
					hydratedByPanel: true,
					panelHydrateCount,
					panelScriptsExecuted
				};
			}

			let dataRows = 0;
			const pairValues = (() => {
				const m = {};
				for (const pair of rawPairs) {
					const k = String((pair && pair[0]) || "").trim();
					if (!k) continue;
					if (!Array.isArray(m[k])) m[k] = [];
					m[k].push(String((pair && pair[1]) || "").trim());
				}
				return m;
			})();
			const at = (name, i) => {
				const arr = pairValues[name];
				if (!Array.isArray(arr) || i < 0 || i >= arr.length) return "";
				return normalize(arr[i] || "");
			};
			const appendRule = (idxValue, boundValue, title, protocol, port, ip, content) => {
				const p = normalize(protocol).toUpperCase();
				const pt = normalize(port);
				const ipAddr = normalize(ip);
				const id = normalize(idxValue);
				if (!p || !pt || !ipAddr) return;
				if (!id) {
					missingIdxCount += 1;
					return;
				}
				const ruleKey = (p + "," + pt + "," + ipAddr).toUpperCase();
				if (seenRuleKeys.has(ruleKey)) return;
				seenRuleKeys.add(ruleKey);
				dataRows += 1;

				const defaultBound = prefix === "inbound" ? "IN" : "OUT";
				fields.push([prefix + "[idx][]", id]);
				fields.push([prefix + "[bound][]", normalize(boundValue).toUpperCase() || defaultBound]);
				fields.push([prefix + "[title][]", normalize(title) || (p + " 직접입력")]);
				fields.push([prefix + "[protocol][]", p]);
				fields.push([prefix + "[port][]", pt]);
				fields.push(["ip", "direct"]);
				fields.push([prefix + "[ip][]", ipAddr]);
				fields.push([prefix + "[unique][]", p + "," + pt + "," + ipAddr]);
				fields.push([prefix + "[content][]", normalize(content)]);
			};

			if (prefix === "inbound" || prefix === "outbound") {
				const names = [
					prefix + "[idx][]",
					prefix + "[bound][]",
					prefix + "[title][]",
					prefix + "[protocol][]",
					prefix + "[port][]",
					prefix + "[ip][]",
					prefix + "[content][]"
				];
				let maxLen = 0;
				for (const name of names) {
					const len = Array.isArray(pairValues[name]) ? pairValues[name].length : 0;
					if (len > maxLen) maxLen = len;
				}
				for (let i = 0; i < maxLen; i++) {
					appendRule(
						at(prefix + "[idx][]", i),
						at(prefix + "[bound][]", i),
						at(prefix + "[title][]", i),
						at(prefix + "[protocol][]", i),
						at(prefix + "[port][]", i),
						at(prefix + "[ip][]", i),
						at(prefix + "[content][]", i)
					);
				}
			}

			const addRuleFromRow = (tr) => {
				const cells = Array.from(tr.querySelectorAll("th,td"))
					.map((el) => normalize(el.innerText || el.textContent || ""))
					.filter(Boolean);
				if (cells.length < 4) return;

				const head = normalize(cells.join(" ")).toUpperCase();
				if (head.includes("서비스") && head.includes("프로토콜")) return;
				if (head.includes("PROTOCOL") && head.includes("PORT")) return;

				const title = normalize(cells[0] || "");
				const protocol = normalize(cells[1] || "").toUpperCase();
				const port = normalize(cells[2] || "");
				const ip = normalize(cells[3] || "");
				const content = normalize(cells[4] || "");
				let idxValue = normalize(tr.getAttribute("data-idx") || "");
				if (!idxValue) {
					const nums = [];
					const pushAttrNums = (el) => {
						if (!el || !el.attributes) return;
						for (const attr of Array.from(el.attributes)) {
							const name = String(attr.name || "").toLowerCase();
							const value = String(attr.value || "");
							if (!value) continue;
							if (
								name.includes("idx") ||
								name === "href" ||
								name.startsWith("on") ||
								name === "value" ||
								name === "data-value" ||
								name === "id"
							) {
								nums.push(...extractNumericTokens(value));
							}
						}
					};
					pushAttrNums(tr);
					for (const child of tr.querySelectorAll("*")) {
						pushAttrNums(child);
					}
					idxValue = pickCandidateIdx(nums, String(idx || "").trim());
				}
				appendRule(idxValue, "", title, protocol, port, ip, content);
			};

			const inferBotTarget = (text) => {
				const t = normalize(text).toUpperCase();
				if (!t) return "";
				if (t.includes("GOOGLE") || t.includes("구글")) return "GOOGLE";
				if (t.includes("NAVER") || t.includes("네이버")) return "NAVER";
				if (t.includes("DAUM") || t.includes("다음")) return "DAUM";
				return "";
			};
			const addBotFromRow = (node) => {
				const target = inferBotTarget(node && (node.innerText || node.textContent || ""));
				if (!target) return false;
				let idxValue = normalize(node.getAttribute("data-idx") || "");
				if (!idxValue) {
					const nums = [];
					const pushAttrNums = (el) => {
						if (!el || !el.attributes) return;
						for (const attr of Array.from(el.attributes)) {
							const name = String(attr.name || "").toLowerCase();
							const value = String(attr.value || "");
							if (!value) continue;
							if (
								name.includes("idx") ||
								name === "href" ||
								name.startsWith("on") ||
								name === "value" ||
								name === "data-value" ||
								name === "id"
							) {
								nums.push(...extractNumericTokens(value));
							}
						}
					};
					pushAttrNums(node);
					for (const child of node.querySelectorAll("*")) {
						pushAttrNums(child);
					}
					idxValue = pickCandidateIdx(nums, String(idx || "").trim());
				}
				fields.push(["bot[idx][]", String(idxValue || "").trim()]);
				fields.push(["bot[target][]", target]);
				dataRows += 1;
				return true;
			};

			if (seenRuleKeys.size === 0 && (prefix === "inbound" || prefix === "outbound")) {
				for (const tr of doc.querySelectorAll("tr")) {
					addRuleFromRow(tr);
				}
			}
			if (prefix === "bot") {
				for (const pair of rawPairs) {
					fields.push(pair);
				}
			}
			if (fields.length === 0 && prefix === "bot") {
				const seen = new Set();
				for (const row of doc.querySelectorAll("tr,li,div")) {
					const before = fields.length;
					if (addBotFromRow(row)) {
						const target = String(fields[fields.length-1]?.[1] || "");
						if (target && seen.has(target)) {
							fields.splice(before, fields.length - before);
							continue;
						}
						if (target) seen.add(target);
					}
				}
			}

			return {
				status: res.status,
				count: fields.length,
				ruleCount: seenRuleKeys.size,
				missingIdxCount,
				fields,
				dataRows,
				bodyPreview: body.slice(0, 240)
			};
		};
		const hydrateSectionIfMissing = async (tab, prefix) => {
			const existingCount = countSectionInputs(prefix);
			diag[prefix + "ExistingCount"] = existingCount;
			if (existingCount > 0) return;

			const fetched = await fetchSectionFields(tab, prefix);
			diag[prefix + "HydrateStatus"] = fetched.status;
			diag[prefix + "HydrateCount"] = fetched.count;
			diag[prefix + "HydrateRuleCount"] = Number(fetched.ruleCount || 0);
			diag[prefix + "HydrateMissingIdxCount"] = Number(fetched.missingIdxCount || 0);
			diag[prefix + "HydrateBodyPreview"] = fetched.bodyPreview;
			diag[prefix + "HydrateByPanel"] = !!fetched.hydratedByPanel;
			diag[prefix + "HydratePanelCount"] = Number(fetched.panelHydrateCount || 0);
			diag[prefix + "HydratePanelScripts"] = Number(fetched.panelScriptsExecuted || 0);
			if (fetched.status !== 200) {
				throw new Error(prefix + " 탭 하이드레이션 실패(status=" + fetched.status + ")");
			}
			if (fetched.hydratedByPanel) {
				return;
			}
			if ((prefix === "inbound" || prefix === "outbound") && Number(fetched.missingIdxCount || 0) > 0) {
				throw new Error(prefix + " 룰 idx를 일부 찾지 못해 안전하게 중단합니다(missingIdx=" + Number(fetched.missingIdxCount || 0) + ")");
			}
			if (!Array.isArray(fetched.fields) || fetched.fields.length === 0) {
				const dataRows = Number(fetched.dataRows || 0);
				diag[prefix + "HydrateDataRows"] = dataRows;
				if (dataRows === 0) {
					diag[prefix + "HydrateSkippedNoRows"] = true;
					return;
				}
				throw new Error(prefix + " 스냅샷을 탭에서 가져오지 못했습니다");
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
			await hydrateSectionIfMissing("bot", "bot");
		} catch (e) {
			return {
				success: false,
				status: 0,
				reason: "기존 정책 스냅샷 하이드레이션 실패: " + String(e),
				bodyHasUnique: false,
				diag
			};
		}

		const ensureRuleUnique = (prefix) => {
			const getNodes = (name) =>
				Array.from(document.querySelectorAll("[name='" + name.replace(/'/g, "\\'") + "']"))
					.filter((el) => !el.form || el.form === form);

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

				if (uniqueNode.value !== uniqueValue) {
					uniqueNode.value = uniqueValue;
				}
				if (uniqueValue) {
					filled += 1;
				}
			}

			return filled;
		};
		diag.inboundUniqueFilled = ensureRuleUnique("inbound");
		diag.outboundUniqueFilled = ensureRuleUnique("outbound");

		const fd = new FormData(form);
		diag.originalInternational = [];
		for (const [k, v] of fd.entries()) {
			if (String(k || "").startsWith("international[")) {
				diag.originalInternational.push(String(k) + "=" + String(v || ""));
			}
		}
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

				if (!idxValue && !protocol && !port && !ip && !titleValue && !content) {
					continue;
				}
				// 편집 폼의 "추가용 빈 템플릿 행"은 전송하지 않는다.
				if (!protocol || !port || !ip) {
					continue;
				}

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
		const collectBotRows = () => {
			const idxs = getScopedValues("bot[idx][]");
			const targets = getScopedValues("bot[target][]");
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
		const botRows = collectBotRows();
		diag.inboundSubmitRows = inboundRows.length;
		diag.outboundSubmitRows = outboundRows.length;
		diag.botSubmitRows = botRows.length;

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
		for (const row of botRows) {
			params.append("bot[idx][]", row.idx);
			params.append("bot[target][]", row.target);
		}
		for (const pair of desiredPairs) {
			params.append("international[idx][]", String(pair.idx || "").trim());
			params.append("international[target][]", String(pair.target || "").trim().toUpperCase());
		}
		diag.submittedInternational = desiredPairs.map((x) => String(x.target || "") + ":" + String(x.idx || ""));
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
		const success = status >= 200 && status < 300 && !looksLikeLogin && !hasErrorKeyword && hasSuccessKeyword;

		return { success, status, bodyHasUnique: false, diag };
	}`, []interface{}{idx, targets, titleFallback})
	if err != nil {
		return false, nil, fmt.Errorf("국제망 정책 저장 스크립트 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("국제망 정책 저장 응답 형식을 해석할 수 없습니다")
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
		return false, diag, fmt.Errorf("국제망 정책 저장 실패 (status=%d, body=%q)", status, strings.TrimSpace(preview))
	}

	return true, diag, nil
}
