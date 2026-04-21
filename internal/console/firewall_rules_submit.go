package console

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"strings"
)

func submitFirewallRuleAdd(page playwright.Page, idx, tab, bound, protocol, port, ruleIP, title, memo string) (bool, map[string]interface{}, error) {
	raw, err := page.Evaluate(`async ([idx, tab, bound, protocol, port, ruleIP, title, memo]) => {
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
		ensureHidden("title", getValue("title") || "");
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
			}
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

		const unique = protocol + "," + port + "," + ruleIP;
		const uniqueWithMask = ruleIP.includes("/") ? unique : protocol + "," + port + "," + ruleIP + "/32";
		const uniqueNoMask = ruleIP.endsWith("/32") ? (protocol + "," + port + "," + ruleIP.slice(0, -3)) : "";
		const zeroAlt = ruleIP === "0.0.0.0/0"
			? (protocol + "," + port + ",0.0.0.0")
			: (ruleIP === "0.0.0.0" ? (protocol + "," + port + ",0.0.0.0/0") : "");
		const existingUnique = collectValues(tab + "[unique][]");
		diag.existingUniqueCount = existingUnique.length;
		diag.existingUniqueTail = existingUnique.slice(-8);
		diag.addUnique = unique;
		diag.addUniqueMask = uniqueWithMask;
		if (
			existingUnique.includes(unique) ||
			existingUnique.includes(uniqueWithMask) ||
			(uniqueNoMask && existingUnique.includes(uniqueNoMask)) ||
			(zeroAlt && existingUnique.includes(zeroAlt))
		) {
			return {
				success: false,
				status: 0,
				reason: "동일한 룰이 이미 존재합니다.",
				unique,
				uniqueWithMask,
				bodyHasUnique: true,
				diag
			};
		}

		appendHidden(tab + "[idx][]", "");
		appendHidden(tab + "[bound][]", bound);
		appendHidden(tab + "[title][]", title);
		appendHidden(tab + "[protocol][]", protocol);
		appendHidden(tab + "[port][]", port);
		appendHidden("ip", "direct");
		appendHidden(tab + "[ip][]", ruleIP);
		appendHidden(tab + "[unique][]", unique);
		appendHidden(tab + "[content][]", memo || "");

		const fd = new FormData(form);
		const params = new URLSearchParams();
		for (const [k, v] of fd.entries()) {
			params.append(k, String(v));
		}

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
		const bodyHasUnique = body.includes(unique) || body.includes(uniqueWithMask);
		const looksLikeLogin = (lower.includes("input[name='id']") || lower.includes("name=\"id\"")) && lower.includes("name=\"pw\"");
		const hasErrorKeyword = lower.includes("오류") || lower.includes("error") || lower.includes("exception") || lower.includes("중복");
		const success = status >= 200 && status < 300 && !looksLikeLogin && !hasErrorKeyword;

		return { success, status, bodyHasUnique, diag };
	}`, []interface{}{idx, tab, bound, protocol, port, ruleIP, title, memo})
	if err != nil {
		return false, nil, fmt.Errorf("방화벽 룰 저장 스크립트 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("방화벽 룰 저장 응답 형식을 해석할 수 없습니다")
	}

	success, _ := res["success"].(bool)
	bodyHasUnique, _ := res["bodyHasUnique"].(bool)
	diag, _ := res["diag"].(map[string]interface{})
	if !success {
		status := toInt(res["status"])
		reason, _ := res["reason"].(string)
		missing := stringifyJSArray(res["missing"])
		unique, _ := res["unique"].(string)
		uniqueWithMask, _ := res["uniqueWithMask"].(string)
		if reason != "" {
			if len(missing) > 0 {
				return bodyHasUnique, diag, fmt.Errorf("방화벽 룰 저장 실패: %s (누락: %s)", reason, strings.Join(missing, ", "))
			}
			if unique != "" || uniqueWithMask != "" {
				return bodyHasUnique, diag, fmt.Errorf("방화벽 룰 저장 실패: %s (candidate=%s, candidate(/32)=%s)", reason, unique, uniqueWithMask)
			}
			return bodyHasUnique, diag, fmt.Errorf("방화벽 룰 저장 실패: %s", reason)
		}
		preview := ""
		if p, ok := diag["bodyPreview"].(string); ok {
			preview = p
		}
		return bodyHasUnique, diag, fmt.Errorf("방화벽 룰 저장 실패 (status=%d, body=%q)", status, strings.TrimSpace(preview))
	}

	return bodyHasUnique, diag, nil
}

func submitFirewallRuleRemove(page playwright.Page, idx, tab, protocol, port, ruleIP string) (bool, map[string]interface{}, error) {
	raw, err := page.Evaluate(`async ([idx, tab, protocol, port, ruleIP]) => {
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
		const collectValues = (name) => {
			return Array.from(document.querySelectorAll("[name='" + name.replace(/'/g, "\\'") + "']"))
				.filter((el) => !el.form || el.form === form)
				.map((el) => String(el.value || "").trim())
				.filter(Boolean);
		};
		const collectRawValues = (name) => {
			return Array.from(document.querySelectorAll("[name='" + name.replace(/'/g, "\\'") + "']"))
				.filter((el) => !el.form || el.form === form)
				.map((el) => String(el.value || "").trim());
		};

		ensureHidden("_token", getValue("_token") || (document.querySelector('meta[name="csrf-token"]')?.content || ""));
		ensureHidden("idx", idx);
		ensureHidden("revision", getValue("revision") || "0");
		ensureHidden("firewallPolicyUnit", getValue("firewallPolicyUnit") || "100");
		ensureHidden("title", getValue("title") || "");
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
			}
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

		const unique = protocol + "," + port + "," + ruleIP;
		const uniqueWithMask = ruleIP.includes("/") ? unique : protocol + "," + port + "," + ruleIP + "/32";
		const uniqueName = tab + "[unique][]";
		const uniqueInputs = Array.from(document.querySelectorAll("[name='" + uniqueName.replace(/'/g, "\\'") + "']"))
			.filter((el) => !el.form || el.form === form);
		const existingUnique = uniqueInputs.map((el) => String(el.value || "").trim()).filter(Boolean);
		diag.existingUniqueCount = existingUnique.length;
		diag.existingUniqueTail = existingUnique.slice(-8);
		diag.removeUnique = unique;
		diag.removeUniqueMask = uniqueWithMask;

		const normalizeProtocol = (value) => String(value || "").trim().toUpperCase();
		const normalizePort = (value) => String(value || "").trim();
		const normalizeIP = (value) => {
			const raw = String(value || "").trim();
			if (!raw) return "";
			if (raw === "0.0.0.0" || raw === "0.0.0.0/0") return "0.0.0.0/0";
			if (raw.includes("/")) return raw;
			if (/^(?:\d{1,3}\.){3}\d{1,3}$/.test(raw)) return raw + "/32";
			return raw;
		};

		const removeIndexSet = new Set();
		for (let i = 0; i < uniqueInputs.length; i++) {
			const value = String(uniqueInputs[i].value || "").trim();
			if (!value) continue;
			if (value === unique || value === uniqueWithMask) {
				removeIndexSet.add(i);
			}
		}
		diag.removeMatchMode = removeIndexSet.size > 0 ? "unique" : "";

		if (removeIndexSet.size === 0) {
			const protocols = collectRawValues(tab + "[protocol][]");
			const ports = collectRawValues(tab + "[port][]");
			const ips = collectRawValues(tab + "[ip][]");
			const rowLen = Math.max(protocols.length, ports.length, ips.length);
			const targetProto = normalizeProtocol(protocol);
			const targetPort = normalizePort(port);
			const targetIP = normalizeIP(ruleIP);
			diag.removeRowCandidateCount = rowLen;
			diag.removeTargetNormalized = targetProto + "," + targetPort + "," + targetIP;

			for (let i = 0; i < rowLen; i++) {
				const rowProto = normalizeProtocol(protocols[i] || "");
				const rowPort = normalizePort(ports[i] || "");
				const rowIPRaw = String(ips[i] || "").trim();
				const rowIP = normalizeIP(rowIPRaw);

				const ipMatch = rowIP === targetIP ||
					(rowIPRaw && String(ruleIP || "").trim() && (rowIPRaw.includes(String(ruleIP || "").trim()) || String(ruleIP || "").trim().includes(rowIPRaw)));
				if (rowProto === targetProto && rowPort === targetPort && ipMatch) {
					removeIndexSet.add(i);
				}
			}
			if (removeIndexSet.size > 0) {
				diag.removeMatchMode = "row";
			}
		}

		const removeIndexes = Array.from(removeIndexSet).sort((a, b) => a - b);
		diag.removeIndexes = removeIndexes.slice();
		if (removeIndexes.length === 0) {
			return {
				success: false,
				status: 0,
				reason: "삭제할 룰을 찾지 못했습니다.",
				unique,
				uniqueWithMask,
				bodyHasUnique: false,
				diag
			};
		}

		const protocolsBefore = collectRawValues(tab + "[protocol][]");
		const portsBefore = collectRawValues(tab + "[port][]");
		const ipsBefore = collectRawValues(tab + "[ip][]");
		diag.removedRows = removeIndexes.map((i) => ({
			index: i,
			protocol: String(protocolsBefore[i] || "").trim().toUpperCase(),
			port: String(portsBefore[i] || "").trim(),
			ip: String(ipsBefore[i] || "").trim()
		}));

		const sectionFieldNames = Array.from(
			new Set(
				Array.from(document.querySelectorAll("[name^='" + tab.replace(/'/g, "\\'") + "[']"))
					.filter((el) => !el.form || el.form === form)
					.map((el) => String(el.getAttribute("name") || "").trim())
					.filter((name) => name.endsWith("[]"))
			)
		);
		diag.sectionFieldCount = sectionFieldNames.length;
		diag.sectionFieldNames = sectionFieldNames.slice(0, 80);

		const sectionCountsBefore = {};
		for (const name of sectionFieldNames) {
			const nodes = Array.from(document.querySelectorAll("[name='" + name.replace(/'/g, "\\'") + "']"))
				.filter((el) => !el.form || el.form === form);
			sectionCountsBefore[name] = nodes.length;
			for (let n = removeIndexes.length - 1; n >= 0; n--) {
				const idxToRemove = removeIndexes[n];
				if (idxToRemove < 0 || idxToRemove >= nodes.length) continue;
				nodes[idxToRemove].remove();
			}
		}
		const sectionCountsAfter = {};
		for (const name of sectionFieldNames) {
			const nodes = Array.from(document.querySelectorAll("[name='" + name.replace(/'/g, "\\'") + "']"))
				.filter((el) => !el.form || el.form === form);
			sectionCountsAfter[name] = nodes.length;
		}
		diag.sectionCountsBefore = sectionCountsBefore;
		diag.sectionCountsAfter = sectionCountsAfter;
		diag.removedCount = removeIndexes.length;

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
				const protocolValue = rowAt(protocols, i).toUpperCase();
				const portValue = rowAt(ports, i);
				const ipValue = rowAt(ips, i);
				const contentValue = rowAt(contents, i);
				if (!idxValue && !protocolValue && !portValue && !ipValue && !titleValue && !contentValue) continue;
				if (!protocolValue || !portValue || !ipValue) continue;
				rows.push({
					idx: idxValue,
					bound: boundValue,
					title: titleValue || (protocolValue + " 직접입력"),
					protocol: protocolValue,
					port: portValue,
					ip: ipValue,
					content: contentValue,
					unique: protocolValue + "," + portValue + "," + ipValue
				});
			}
			return rows;
		};
		const collectTargetRows = (prefix, normalizeTarget) => {
			const idxs = getScopedValues(prefix + "[idx][]");
			const targets = getScopedValues(prefix + "[target][]");
			const maxLen = Math.max(idxs.length, targets.length);
			const rows = [];
			for (let i = 0; i < maxLen; i++) {
				const idxValue = rowAt(idxs, i);
				const rawTarget = rowAt(targets, i);
				if (!idxValue && !rawTarget) continue;
				const targetValue = normalizeTarget ? normalizeTarget(rawTarget) : rawTarget;
				if (!targetValue) continue;
				rows.push({
					idx: idxValue,
					target: targetValue
				});
			}
			return rows;
		};

		const inboundRows = collectRuleRows("inbound", "IN");
		const outboundRows = collectRuleRows("outbound", "OUT");
		const internationalRows = collectTargetRows("international", (v) => String(v || "").trim().toUpperCase());
		const botRows = collectTargetRows("bot", (v) => String(v || "").trim().toUpperCase());
		diag.inboundSubmitRows = inboundRows.length;
		diag.outboundSubmitRows = outboundRows.length;
		diag.internationalSubmitRows = internationalRows.length;
		diag.botSubmitRows = botRows.length;

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
		for (const row of botRows) {
			params.append("bot[idx][]", row.idx);
			params.append("bot[target][]", row.target);
		}
		diag.totalRuleRows = inboundRows.length + outboundRows.length;
		const paramEntries = Array.from(params.entries());
		diag.submitParamCount = paramEntries.length;
		diag.submitParamPreview = paramEntries.slice(0, 80).map(([k, v]) => String(k) + "=" + String(v || "").slice(0, 120));
		const submitKeyCounts = {};
		for (const [k] of paramEntries) {
			const key = String(k || "");
			if (!key) continue;
			submitKeyCounts[key] = (submitKeyCounts[key] || 0) + 1;
		}
		diag.submitKeyCounts = submitKeyCounts;

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
		const hasErrorKeyword =
			lower.includes("오류") ||
			lower.includes("error") ||
			lower.includes("exception") ||
			body.includes("일시적인 장애가 발생되고 있습니다.") ||
			body.includes("장애가 발생");
		const hasSuccessKeyword =
			body.includes("정상적으로 적용하였습니다.") ||
			body.includes("정보 수정이 완료되었습니다.");
		diag.looksLikeLogin = looksLikeLogin;
		diag.hasErrorKeyword = hasErrorKeyword;
		diag.hasSuccessKeyword = hasSuccessKeyword;
		const success = status >= 200 && status < 300 && !looksLikeLogin && !hasErrorKeyword && hasSuccessKeyword;

		return { success, status, bodyHasUnique: false, diag };
	}`, []interface{}{idx, tab, protocol, port, ruleIP})
	if err != nil {
		return false, nil, fmt.Errorf("방화벽 룰 삭제 스크립트 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("방화벽 룰 삭제 응답 형식을 해석할 수 없습니다")
	}

	success, _ := res["success"].(bool)
	diag, _ := res["diag"].(map[string]interface{})
	if !success {
		status := toInt(res["status"])
		reason, _ := res["reason"].(string)
		missing := stringifyJSArray(res["missing"])
		unique, _ := res["unique"].(string)
		uniqueWithMask, _ := res["uniqueWithMask"].(string)
		if reason != "" {
			if len(missing) > 0 {
				return false, diag, fmt.Errorf("%s (누락: %s)", reason, strings.Join(missing, ", "))
			}
			if unique != "" || uniqueWithMask != "" {
				return false, diag, fmt.Errorf("%s (candidate=%s, candidate(/32)=%s)", reason, unique, uniqueWithMask)
			}
			return false, diag, fmt.Errorf("%s", reason)
		}
		preview := ""
		if p, ok := diag["bodyPreview"].(string); ok {
			preview = p
		}
		return false, diag, fmt.Errorf("방화벽 룰 삭제 실패 (status=%d, body=%q)", status, strings.TrimSpace(preview))
	}

	return true, diag, nil
}
