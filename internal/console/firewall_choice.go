package console

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

type instanceFirewallChoiceOption struct {
	Value   string
	Label   string
	Checked bool
}

type instanceFirewallChoiceState struct {
	ServerIdx             string
	Token                 string
	TypeValue             string
	UseValue              string
	UseInputMode          string
	SelectedFirewall      string
	SelectedFirewallLabel string
	EditHref              string
	EditFirewallIdx       string
	Options               []instanceFirewallChoiceOption
}

func normalizeFirewallChoiceUse(input string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(input))
	switch normalized {
	case "Y", "ON", "TRUE", "1", "ENABLE", "ENABLED":
		return "Y", nil
	case "N", "OFF", "FALSE", "0", "DISABLE", "DISABLED":
		return "N", nil
	default:
		return "", fmt.Errorf("지원하지 않는 --firewall-choice-use 값입니다: %q (가능값: Y|N|on|off)", input)
	}
}

func RunSetInstanceFirewallChoice(page playwright.Page, serverRef, policyRef, useInput string, debug bool) error {
	useValue, err := normalizeFirewallChoiceUse(useInput)
	if err != nil {
		return err
	}

	serverRef = strings.TrimSpace(serverRef)
	policyRef = strings.TrimSpace(policyRef)
	if serverRef == "" {
		return fmt.Errorf("--firewall-choice-server 값이 비어 있습니다")
	}
	if policyRef == "" {
		return fmt.Errorf("--firewall-choice-policy 값이 비어 있습니다")
	}

	server, err := lookupServer(page, serverRef)
	if err != nil {
		return fmt.Errorf("대상 서버 확인 실패: %w", err)
	}
	serverName := strings.TrimSpace(server.Name)
	if serverName == "" {
		serverName = server.Idx
	}

	before, err := fetchInstanceFirewallChoiceState(page, server.Idx)
	if err != nil {
		return fmt.Errorf("현재 서버 방화벽 선택 상태 조회 실패: %w", err)
	}

	selectedFirewall, selectedFirewallLabel, err := resolveFirewallChoicePolicyRef(before.Options, policyRef)
	if err != nil {
		return err
	}

	firewallDebugf(debug, "firewall choice start | server=%s(%s) firewall=%s(%s) use=%s", serverName, server.Idx, selectedFirewallLabel, selectedFirewall, useValue)
	firewallDebugf(debug, "firewall choice before | use=%s mode=%s selected=%s(%s) editIdx=%s optionCount=%d", before.UseValue, before.UseInputMode, before.SelectedFirewallLabel, before.SelectedFirewall, before.EditFirewallIdx, len(before.Options))
	if debug {
		firewallDebugf(true, "firewall choice options=%v", summarizeChoiceOptions(before.Options, 20))
	}

	typeValue := before.TypeValue
	if strings.TrimSpace(typeValue) == "" {
		if useValue == "Y" {
			typeValue = `{"use":"hidden","firewall":"radio"}`
		} else {
			typeValue = `{"use":"radio","firewall":"radio"}`
		}
	}

	formToken := strings.TrimSpace(before.Token)
	if formToken == "" {
		return fmt.Errorf("방화벽 선택 폼의 _token을 찾지 못했습니다")
	}

	postBody := url.Values{}
	postBody.Set("_token", formToken)
	postBody.Set("idx", strings.TrimSpace(server.Idx))
	postBody.Set("use", useValue)
	postBody.Set("firewall", selectedFirewall)
	postBody.Set("_type", typeValue)

	if debug {
		firewallDebugf(true, "firewall choice submitBody=%q", postBody.Encode())
	}

	resRaw, err := page.Evaluate(`async (payload) => {
		const headers = {
			"accept": "application/json, text/javascript, */*; q=0.01",
			"content-type": "application/x-www-form-urlencoded; charset=UTF-8",
			"x-requested-with": "XMLHttpRequest"
		};

		const csrf = document.querySelector("meta[name='csrf-token']")?.content || payload.token || "";
		if (csrf) {
			headers["x-csrf-token"] = csrf;
		}

		const response = await fetch(payload.url, {
			method: "POST",
			headers,
			body: payload.body,
			credentials: "same-origin"
		});

		const text = await response.text();
		let json = null;
		try {
			json = JSON.parse(text);
		} catch (_err) {
			json = null;
		}

		const lower = text.toLowerCase();
		const looksLikeLogin = lower.includes("name=\"id\"") && lower.includes("name=\"pw\"");
		const hasErrorKeyword =
			lower.includes("오류") ||
			lower.includes("error") ||
			lower.includes("exception") ||
			lower.includes("실패");

		return {
			status: response.status,
			body: text,
			json,
			looksLikeLogin,
			hasErrorKeyword
		};
	}`, map[string]interface{}{
		"url":   firewallURL + "/choice",
		"body":  postBody.Encode(),
		"token": formToken,
	})
	if err != nil {
		return fmt.Errorf("방화벽 선택 저장 요청 실행 실패: %w", err)
	}

	res, ok := resRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("방화벽 선택 저장 응답 형식을 해석할 수 없습니다")
	}

	status := toInt(res["status"])
	bodyText := strings.TrimSpace(toString(res["body"]))
	if debug {
		firewallDebugf(true, "firewall choice status=%d looksLikeLogin=%t hasErrorKeyword=%t", status, toBoolLike(res["looksLikeLogin"]), toBoolLike(res["hasErrorKeyword"]))
		firewallDebugf(true, "firewall choice bodyPreview=%q", previewText(bodyText, 300))
		if jsonObj, ok := res["json"].(map[string]interface{}); ok {
			firewallDebugf(true, "firewall choice json=%v", jsonObj)
		}
	}

	if toBoolLike(res["looksLikeLogin"]) {
		return fmt.Errorf("방화벽 사용 설정 실패: 로그인 세션이 만료되었습니다. `--login`으로 재로그인 후 다시 시도하세요")
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("방화벽 사용 설정 실패 (status=%d, body=%q)", status, previewText(bodyText, 300))
	}

	if jsonObj, ok := res["json"].(map[string]interface{}); ok {
		if resultObj, ok := jsonObj["result"].(map[string]interface{}); ok {
			code := toInt(resultObj["code"])
			message := strings.TrimSpace(toString(resultObj["message"]))
			if code != 0 {
				if message == "" {
					message = "알 수 없는 오류"
				}
				return fmt.Errorf("방화벽 사용 설정 실패 (code=%d, message=%s)", code, message)
			}
		}
	}

	time.Sleep(300 * time.Millisecond)

	after, err := fetchInstanceFirewallChoiceState(page, server.Idx)
	if err != nil {
		return fmt.Errorf("저장 후 서버 방화벽 선택 상태 재조회 실패: %w", err)
	}

	firewallDebugf(debug, "firewall choice after | use=%s mode=%s selected=%s(%s) editIdx=%s optionCount=%d", after.UseValue, after.UseInputMode, after.SelectedFirewallLabel, after.SelectedFirewall, after.EditFirewallIdx, len(after.Options))
	if debug {
		firewallDebugf(true, "firewall choice options(after)=%v", summarizeChoiceOptions(after.Options, 20))
	}

	if strings.ToUpper(strings.TrimSpace(after.UseValue)) != useValue {
		return fmt.Errorf("방화벽 사용 설정 확인 실패: 기대 use=%s, 실제 use=%s", useValue, after.UseValue)
	}
	if useValue == "Y" {
		if strings.TrimSpace(after.SelectedFirewall) != selectedFirewall {
			return fmt.Errorf("방화벽 사용 설정 확인 실패: 기대 firewall=%s(%s), 실제 firewall=%s(%s)", selectedFirewallLabel, selectedFirewall, after.SelectedFirewallLabel, after.SelectedFirewall)
		}
		if strings.TrimSpace(after.EditFirewallIdx) != selectedFirewall {
			return fmt.Errorf("방화벽 사용 설정 확인 실패: 변경 링크 IDX 불일치 (기대:%s, 실제:%s, href:%s)", selectedFirewall, after.EditFirewallIdx, after.EditHref)
		}
	} else {
		if strings.TrimSpace(after.SelectedFirewall) != "" {
			return fmt.Errorf("방화벽 사용 설정 확인 실패: OFF 기대값과 다르게 선택된 정책이 남아 있습니다 (selected=%s)", after.SelectedFirewall)
		}
		if strings.TrimSpace(after.EditFirewallIdx) != "0" {
			return fmt.Errorf("방화벽 사용 설정 확인 실패: OFF 기대값과 다르게 변경 링크 IDX가 0이 아닙니다 (idx=%s, href=%s)", after.EditFirewallIdx, after.EditHref)
		}
	}

	if useValue == "Y" {
		fmt.Printf("✅ [서버:%s | IDX:%s] 방화벽 사용 설정 완료: use=Y, firewall=%s(IDX:%s)\n", serverName, server.Idx, selectedFirewallLabel, selectedFirewall)
	} else {
		fmt.Printf("✅ [서버:%s | IDX:%s] 방화벽 사용 설정 완료: use=N\n", serverName, server.Idx)
	}

	return nil
}

func fetchInstanceFirewallChoiceState(page playwright.Page, serverIdx string) (instanceFirewallChoiceState, error) {
	idx := strings.TrimSpace(serverIdx)
	if idx == "" {
		return instanceFirewallChoiceState{}, fmt.Errorf("서버 IDX가 비어 있습니다")
	}

	choiceURL := fmt.Sprintf("%s/choice?idx=%s&_=%d", firewallURL, url.QueryEscape(idx), time.Now().UnixNano())
	if _, err := page.Goto(choiceURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return instanceFirewallChoiceState{}, fmt.Errorf("방화벽 선택 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "방화벽 선택 페이지 접속"); err != nil {
		return instanceFirewallChoiceState{}, err
	}

	raw, err := page.Evaluate(`(serverIdx) => {
		const normalize = (text) => String(text || "").replace(/\s+/g, " ").trim();
		const readLabel = (input) => {
			if (!input) return "";
			let text = "";
			const wrapper = input.closest("label");
			if (wrapper && wrapper.innerText) {
				text = wrapper.innerText;
			}
			if (!text && input.id) {
				const direct = document.querySelector("label[for='" + input.id.replace(/'/g, "\\'") + "']");
				if (direct && direct.innerText) {
					text = direct.innerText;
				}
			}
			if (!text) {
				const parent = input.parentElement;
				if (parent && parent.innerText) {
					text = parent.innerText;
				}
			}
			text = normalize(text);
			if (!text) {
				return "";
			}
			const pieces = text.split(" ");
			if (pieces.length > 1 && pieces[0].replace(/\D+/g, "") === "") {
				return text;
			}
			return text;
		};

		const form =
			document.querySelector("form[action*='/firewall/choice']") ||
			document.querySelector("form[name='modal']") ||
			document.querySelector("main form");

		const token =
			(form && form.querySelector("input[name='_token']") && form.querySelector("input[name='_token']").value) ||
			document.querySelector("meta[name='csrf-token']")?.content ||
			"";

		const typeValue =
			(form && form.querySelector("input[name='_type']") && form.querySelector("input[name='_type']").value) ||
			"";

		let useValue = "";
		let useInputMode = "";
		const useNodes = Array.from(document.querySelectorAll("input[name='use']"));
		for (const node of useNodes) {
			if (!node) continue;
			if (!useInputMode) {
				useInputMode = normalize(node.type || "");
			}
			if (node && node.checked) {
				useValue = normalize(node.value).toUpperCase();
				break;
			}
		}

		let options = [];
		let selectedFirewall = "";
		let selectedFirewallLabel = "";
		const firewallNodes = Array.from(document.querySelectorAll("input[name='firewall']"));
		for (const node of firewallNodes) {
			if (!node) continue;
			const value = normalize(node.value);
			if (!value) continue;
			const label = readLabel(node);
			const checked = !!node.checked;
			options.push({ value, label, checked });
			if (checked) {
				selectedFirewall = value;
				selectedFirewallLabel = label;
			}
		}

		const editAnchorCandidates = Array.from(document.querySelectorAll("a[data-href], a[href]"));
		let editHref = "";
		for (const a of editAnchorCandidates) {
			if (!a) continue;
			const text = normalize(a.innerText || "");
			const rawHref = normalize(a.getAttribute("data-href") || a.getAttribute("href") || "");
			if (!rawHref) continue;
			if (!(text.includes("ELCAP") && text.includes("변경"))) continue;
			editHref = rawHref;
			break;
		}
		let editFirewallIdx = "";
		if (editHref) {
			try {
				const abs = new URL(editHref, location.origin);
				editFirewallIdx = normalize(abs.searchParams.get("idx") || "");
			} catch (_e) {
				editFirewallIdx = "";
			}
		}

		const idxValue =
			(form && form.querySelector("input[name='idx']") && normalize(form.querySelector("input[name='idx']").value)) ||
			normalize(serverIdx);

		if (!useValue) {
			if (selectedFirewall) {
				useValue = "Y";
			} else if (editFirewallIdx === "0") {
				useValue = "N";
			} else if (useInputMode === "hidden" && !selectedFirewall) {
				useValue = "N";
			}
		}

		return {
			serverIdx: idxValue,
			token,
			typeValue,
			useValue,
			useInputMode,
			selectedFirewall,
			selectedFirewallLabel,
			editHref,
			editFirewallIdx,
			options,
		};
	}`, idx)
	if err != nil {
		return instanceFirewallChoiceState{}, fmt.Errorf("방화벽 선택 상태 파싱 스크립트 실행 실패: %w", err)
	}

	obj, ok := raw.(map[string]interface{})
	if !ok {
		return instanceFirewallChoiceState{}, fmt.Errorf("방화벽 선택 상태 응답 형식을 해석할 수 없습니다")
	}

	state := instanceFirewallChoiceState{
		ServerIdx:             strings.TrimSpace(toString(obj["serverIdx"])),
		Token:                 strings.TrimSpace(toString(obj["token"])),
		TypeValue:             strings.TrimSpace(toString(obj["typeValue"])),
		UseValue:              strings.ToUpper(strings.TrimSpace(toString(obj["useValue"]))),
		UseInputMode:          strings.ToLower(strings.TrimSpace(toString(obj["useInputMode"]))),
		SelectedFirewall:      strings.TrimSpace(toString(obj["selectedFirewall"])),
		SelectedFirewallLabel: strings.TrimSpace(toString(obj["selectedFirewallLabel"])),
		EditHref:              strings.TrimSpace(toString(obj["editHref"])),
		EditFirewallIdx:       strings.TrimSpace(toString(obj["editFirewallIdx"])),
	}
	if state.ServerIdx == "" {
		state.ServerIdx = idx
	}

	if optionItems, ok := obj["options"].([]interface{}); ok {
		state.Options = make([]instanceFirewallChoiceOption, 0, len(optionItems))
		for _, item := range optionItems {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			value := strings.TrimSpace(toString(m["value"]))
			if value == "" {
				continue
			}
			state.Options = append(state.Options, instanceFirewallChoiceOption{
				Value:   value,
				Label:   strings.TrimSpace(toString(m["label"])),
				Checked: toBoolLike(m["checked"]),
			})
		}
	}

	if len(state.Options) == 0 {
		return instanceFirewallChoiceState{}, fmt.Errorf("방화벽 선택 옵션을 찾지 못했습니다 (server idx: %s)", idx)
	}

	if state.UseValue == "" {
		for _, opt := range state.Options {
			if opt.Checked {
				state.UseValue = "Y"
				break
			}
		}
	}

	return state, nil
}

func resolveFirewallChoicePolicyRef(options []instanceFirewallChoiceOption, policyRef string) (string, string, error) {
	ref := strings.TrimSpace(policyRef)
	if ref == "" {
		return "", "", fmt.Errorf("--firewall-choice-policy 값이 비어 있습니다")
	}
	if len(options) == 0 {
		return "", "", fmt.Errorf("선택 가능한 방화벽 목록이 비어 있습니다")
	}

	if numericIDRegex.MatchString(ref) {
		for _, option := range options {
			if strings.TrimSpace(option.Value) == ref {
				label := strings.TrimSpace(option.Label)
				if label == "" {
					label = option.Value
				}
				return option.Value, label, nil
			}
		}
		return "", "", fmt.Errorf("서버에 적용 가능한 방화벽 목록에 IDX %s가 없습니다. (가능값: %s)", ref, strings.Join(summarizeChoiceOptions(options, 15), " | "))
	}

	needle := strings.ToLower(ref)
	matches := make([]instanceFirewallChoiceOption, 0, 4)
	for _, option := range options {
		label := strings.ToLower(strings.TrimSpace(option.Label))
		value := strings.ToLower(strings.TrimSpace(option.Value))
		if strings.Contains(label, needle) || value == needle {
			matches = append(matches, option)
		}
	}

	if len(matches) == 0 {
		return "", "", fmt.Errorf("서버에 적용 가능한 방화벽 목록에서 '%s'을(를) 찾지 못했습니다. (가능값: %s)", ref, strings.Join(summarizeChoiceOptions(options, 15), " | "))
	}
	if len(matches) > 1 {
		return "", "", fmt.Errorf("방화벽 '%s'이(가) 여러 개입니다: %s", ref, strings.Join(summarizeChoiceOptions(matches, 15), " | "))
	}

	label := strings.TrimSpace(matches[0].Label)
	if label == "" {
		label = matches[0].Value
	}
	return matches[0].Value, label, nil
}

func summarizeChoiceOptions(options []instanceFirewallChoiceOption, max int) []string {
	if len(options) == 0 {
		return nil
	}
	if max <= 0 {
		max = len(options)
	}
	summary := make([]string, 0, len(options))
	for _, opt := range options {
		label := strings.TrimSpace(opt.Label)
		if label == "" {
			label = "(이름없음)"
		}
		mark := ""
		if opt.Checked {
			mark = "*"
		}
		summary = append(summary, fmt.Sprintf("%s%s(IDX:%s)", mark, label, strings.TrimSpace(opt.Value)))
	}
	sort.Strings(summary)
	if len(summary) > max {
		return append(summary[:max], fmt.Sprintf("...(+%d)", len(summary)-max))
	}
	return summary
}

func toBoolLike(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		s := strings.TrimSpace(strings.ToLower(val))
		return s == "true" || s == "1" || s == "y" || s == "yes"
	case float64:
		return val != 0
	case int:
		return val != 0
	default:
		return false
	}
}

func previewText(text string, max int) string {
	if max <= 0 {
		max = 300
	}
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max] + "..."
}
