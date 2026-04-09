package console

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/playwright-community/playwright-go"
)

type FirewallPolicy struct {
	Idx  string
	Name string
	Raw  string
}

type firewallDirectionTarget struct {
	Tab   string
	Bound string
	Label string
}

type firewallFormSectionCounts struct {
	Title              string
	InboundCount       int
	OutboundCount      int
	InternationalCount int
	BotCount           int
}

var numericIDRegex = regexp.MustCompile(`^\d+$`)

var firewallInternationalAliasMap = map[string]string{
	"FOREIGN":   "FOREIGN",
	"해외":        "FOREIGN",
	"해외전체":      "FOREIGN",
	"한국제외":      "FOREIGN",
	"KR-EXCEPT": "FOREIGN",
	"KR_EXCEPT": "FOREIGN",
	"ALL":       "FOREIGN",

	"TAIWAN": "TAIWAN",
	"대만":     "TAIWAN",

	"CHINA": "CHINA",
	"중국":    "CHINA",

	"PHILIPPINES": "PHILIPPINES",
	"PHILIPPINE":  "PHILIPPINES",
	"필리핀":         "PHILIPPINES",

	"USA": "USA",
	"US":  "USA",
	"미국":  "USA",

	"JAPAN": "JAPAN",
	"일본":    "JAPAN",
}

var firewallInternationalAllowed = []string{
	"FOREIGN",
	"TAIWAN",
	"CHINA",
	"PHILIPPINES",
	"USA",
	"JAPAN",
}

var firewallInternationalDisplayHints = map[string][]string{
	"FOREIGN": {"FOREIGN", "해외", "한국제외", "한국 제외", "한국만 허용", "국내만 허용"},
	"TAIWAN":  {"TAIWAN", "대만"},
	"CHINA":   {"CHINA", "중국"},
	"PHILIPPINES": {
		"PHILIPPINES",
		"PHILIPPINE",
		"필리핀",
	},
	"USA":   {"USA", "US", "미국"},
	"JAPAN": {"JAPAN", "일본"},
}

var firewallBotAliasMap = map[string]string{
	"ALL":     "ALL",
	"전체":      "ALL",
	"ALLBOT":  "ALL",
	"ALL_BOT": "ALL",

	"GOOGLE": "GOOGLE",
	"구글":     "GOOGLE",

	"NAVER": "NAVER",
	"네이버":   "NAVER",

	"DAUM": "DAUM",
	"다음":   "DAUM",
}

var firewallBotAllowed = []string{
	"ALL",
	"GOOGLE",
	"NAVER",
	"DAUM",
}

var firewallBotDisplayHints = map[string][]string{
	"ALL":    {"ALL", "전체", "모든", "전체 Bot", "전체봇"},
	"GOOGLE": {"GOOGLE", "구글"},
	"NAVER":  {"NAVER", "네이버"},
	"DAUM":   {"DAUM", "다음"},
}

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

func RunShowFirewallTab(page playwright.Page, tabInput, policyRef string) error {
	tab, tabLabel, err := normalizeFirewallTab(tabInput)
	if err != nil {
		return err
	}

	fmt.Printf("🚀 ELCAP %s 정책을 조회 중입니다...\n", tabLabel)

	ref := strings.TrimSpace(policyRef)
	idx := ref
	resolvedName := ref
	if !numericIDRegex.MatchString(ref) {
		if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
		}
		if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
			return err
		}

		resolvedIdx, resolvedPolicyName, err := resolveFirewallPolicyIdx(page, ref)
		if err != nil {
			return err
		}
		idx = resolvedIdx
		resolvedName = resolvedPolicyName
	}

	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabPageURL, url.QueryEscape(idx))
	if _, err := page.Goto(tabPageURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("방화벽 정책 상세 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "방화벽 정책 상세 페이지 접속"); err != nil {
		return err
	}

	rows, err := fetchFirewallTabRows(page, tab, idx)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		fmt.Printf("❌ [%s] 정책에서 '%s' 탭 데이터가 없습니다.\n", idx, tabLabel)
		return nil
	}

	if resolvedName == "" {
		resolvedName = policyRef
	}

	printFirewallTabRows(tabLabel, resolvedName, idx, rows)
	return nil
}

func RunAddFirewallRule(page playwright.Page, policyRef, direction, protocol, port, ruleIP, title, memo string, debug bool) error {
	directions, err := normalizeFirewallDirections(direction)
	if err != nil {
		return err
	}

	protocolNorm, err := normalizeFirewallProtocol(protocol)
	if err != nil {
		return err
	}

	port = strings.TrimSpace(port)
	ruleIP = normalizeRuleIPInput(ruleIP)
	if port == "" || ruleIP == "" {
		return fmt.Errorf("포트와 IP는 비어 있을 수 없습니다")
	}

	title = strings.TrimSpace(title)
	if title == "" {
		title = protocolNorm + " 직접입력"
	}

	ref := strings.TrimSpace(policyRef)
	idx := ref
	resolvedName := ref
	if !numericIDRegex.MatchString(ref) {
		if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
		}
		if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
			return err
		}

		resolvedIdx, resolvedPolicyName, err := resolveFirewallPolicyIdx(page, ref)
		if err != nil {
			return err
		}
		idx = resolvedIdx
		resolvedName = resolvedPolicyName
		firewallDebugf(debug, "resolved policy | name=%s idx=%s", resolvedName, idx)
	}

	for _, target := range directions {
		fmt.Printf("🚀 ELCAP %s 룰 추가 중... (%s %s %s)\n", target.Label, protocolNorm, port, ruleIP)
		firewallDebugf(debug, "start add | ref=%s tab=%s bound=%s proto=%s port=%s ip=%s", policyRef, target.Tab, target.Bound, protocolNorm, port, ruleIP)

		editPageURL := fmt.Sprintf("%s/%s/edit?tab=%s", firewallURL, url.PathEscape(idx), url.QueryEscape(target.Tab))
		firewallDebugf(debug, "goto edit page | url=%s", editPageURL)
		if _, err := page.Goto(editPageURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return fmt.Errorf("방화벽 정책 편집 페이지 접속 실패: %w", err)
		}
		if err := ensureAuthenticated(page, "방화벽 정책 편집 페이지 접속"); err != nil {
			return err
		}
		if err := waitForFirewallWriteFormReady(page, 8*time.Second); err != nil {
			return err
		}
		firewallDebugf(debug, "write form ready")

		// 추가 전 현재 탭을 재조회해 동일 룰 중복 생성을 사전 차단한다.
		preRows, preErr := fetchFirewallTabRows(page, target.Tab, idx)
		if preErr == nil && hasRuleRow(preRows, protocolNorm, port, ruleIP) {
			fmt.Printf("ℹ️ [%s | IDX:%s] %s 룰은 이미 존재하여 추가를 건너뜁니다: %s %s %s\n", strings.TrimSpace(resolvedName), idx, target.Label, protocolNorm, port, ruleIP)
			firewallDebugf(debug, "duplicate precheck hit in tab rows | tab=%s rows=%d", target.Tab, len(preRows))
			continue
		}
		if preErr != nil {
			firewallDebugf(debug, "duplicate precheck tab fetch error | %v", preErr)
		}

		existsInForm, formErr := hasRuleUniqueInForm(page, target.Tab, protocolNorm, port, ruleIP)
		if formErr == nil && existsInForm {
			fmt.Printf("ℹ️ [%s | IDX:%s] %s 룰은 이미 존재하여 추가를 건너뜁니다: %s %s %s\n", strings.TrimSpace(resolvedName), idx, target.Label, protocolNorm, port, ruleIP)
			firewallDebugf(debug, "duplicate precheck hit in form unique | tab=%s", target.Tab)
			continue
		}
		if formErr != nil {
			firewallDebugf(debug, "duplicate precheck form error | %v", formErr)
		}

		_, submitDiag, err := submitFirewallRuleAdd(page, idx, target.Tab, target.Bound, protocolNorm, port, ruleIP, title, memo)
		if err != nil {
			firewallDebugf(debug, "submit failed | %v", err)
			logFirewallSubmitDiagnostics(debug, submitDiag)
			return fmt.Errorf("%s 룰 저장 실패: %w", target.Label, err)
		}
		logFirewallSubmitDiagnostics(debug, submitDiag)

		ok, verifyErr := waitForFirewallRuleApplied(page, idx, target.Tab, protocolNorm, port, ruleIP, 28*time.Second, debug)
		if ok {
			fmt.Printf("✅ [%s | IDX:%s] %s 룰 추가 완료: %s %s %s\n", strings.TrimSpace(resolvedName), idx, target.Label, protocolNorm, port, ruleIP)
			firewallDebugf(debug, "verify success")
		} else {
			firewallDebugf(debug, "verify failed | %v", verifyErr)
			if verifyErr != nil {
				return fmt.Errorf("%s 룰 추가 후 검증 실패: %w", target.Label, verifyErr)
			}
			return fmt.Errorf("%s 룰 추가 후 반영을 확인하지 못했습니다. --firewall-tab %s --firewall-ref \"%s\"로 재확인하세요", target.Label, target.Tab, idx)
		}
	}

	return nil
}

func RunRemoveFirewallRule(page playwright.Page, policyRef, direction, protocol, port, ruleIP string, debug bool) error {
	directions, err := normalizeFirewallDirections(direction)
	if err != nil {
		return err
	}

	protocolNorm, err := normalizeFirewallProtocol(protocol)
	if err != nil {
		return err
	}

	port = strings.TrimSpace(port)
	ruleIP = normalizeRuleIPInput(ruleIP)
	if port == "" || ruleIP == "" {
		return fmt.Errorf("포트와 IP는 비어 있을 수 없습니다")
	}

	ref := strings.TrimSpace(policyRef)
	idx := ref
	resolvedName := ref
	if !numericIDRegex.MatchString(ref) {
		if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
		}
		if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
			return err
		}

		resolvedIdx, resolvedPolicyName, err := resolveFirewallPolicyIdx(page, ref)
		if err != nil {
			return err
		}
		idx = resolvedIdx
		resolvedName = resolvedPolicyName
		firewallDebugf(debug, "resolved policy | name=%s idx=%s", resolvedName, idx)
	}

	for _, target := range directions {
		fmt.Printf("🗑️ ELCAP %s 룰 삭제 중... (%s %s %s)\n", target.Label, protocolNorm, port, ruleIP)
		firewallDebugf(debug, "start remove | ref=%s tab=%s proto=%s port=%s ip=%s", policyRef, target.Tab, protocolNorm, port, ruleIP)

		editPageURL := fmt.Sprintf("%s/%s/edit?tab=%s", firewallURL, url.PathEscape(idx), url.QueryEscape(target.Tab))
		firewallDebugf(debug, "goto edit page | url=%s", editPageURL)
		if _, err := page.Goto(editPageURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return fmt.Errorf("방화벽 정책 편집 페이지 접속 실패: %w", err)
		}
		if err := ensureAuthenticated(page, "방화벽 정책 편집 페이지 접속"); err != nil {
			return err
		}
		if err := waitForFirewallWriteFormReady(page, 8*time.Second); err != nil {
			return err
		}
		firewallDebugf(debug, "write form ready")

		preRows, preErr := fetchFirewallTabRows(page, target.Tab, idx)
		if preErr == nil && !hasRuleRow(preRows, protocolNorm, port, ruleIP) {
			fmt.Printf("ℹ️ [%s | IDX:%s] %s 룰이 없어 삭제를 건너뜁니다: %s %s %s\n", strings.TrimSpace(resolvedName), idx, target.Label, protocolNorm, port, ruleIP)
			firewallDebugf(debug, "remove precheck no match in tab rows | tab=%s rows=%d", target.Tab, len(preRows))
			continue
		}
		if preErr != nil {
			firewallDebugf(debug, "remove precheck tab fetch error | %v", preErr)
		}

		existsInForm, formErr := hasRuleUniqueInForm(page, target.Tab, protocolNorm, port, ruleIP)
		if formErr == nil && !existsInForm && preErr == nil {
			// 일부 정책은 화면 표에는 보이지만 unique hidden 필드가 비어 있다.
			// 이 경우 submit 단계에서 protocol/port/ip 기준 fallback 삭제를 시도한다.
			firewallDebugf(debug, "remove precheck no match in form unique | tab=%s (continue with row fallback)", target.Tab)
		}
		if formErr != nil {
			firewallDebugf(debug, "remove precheck form error | %v", formErr)
		}

		_, submitDiag, err := submitFirewallRuleRemove(page, idx, target.Tab, protocolNorm, port, ruleIP)
		if err != nil {
			firewallDebugf(debug, "remove submit failed | %v", err)
			logFirewallSubmitDiagnostics(debug, submitDiag)
			return fmt.Errorf("%s 룰 삭제 실패: %w", target.Label, err)
		}
		logFirewallSubmitDiagnostics(debug, submitDiag)

		ok, verifyErr := waitForFirewallRuleRemoved(page, idx, target.Tab, protocolNorm, port, ruleIP, 28*time.Second, debug)
		if ok {
			fmt.Printf("✅ [%s | IDX:%s] %s 룰 삭제 완료: %s %s %s\n", strings.TrimSpace(resolvedName), idx, target.Label, protocolNorm, port, ruleIP)
			firewallDebugf(debug, "remove verify success")
		} else {
			firewallDebugf(debug, "remove verify failed | %v", verifyErr)
			if verifyErr != nil {
				return fmt.Errorf("%s 룰 삭제 후 검증 실패: %w", target.Label, verifyErr)
			}
			return fmt.Errorf("%s 룰 삭제 후 반영을 확인하지 못했습니다. --firewall-tab %s --firewall-ref \"%s\"로 재확인하세요", target.Label, target.Tab, idx)
		}
	}

	return nil
}

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

	ref := strings.TrimSpace(policyRef)
	idx := ref
	resolvedName := ref
	if !numericIDRegex.MatchString(ref) {
		if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
		}
		if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
			return err
		}

		resolvedIdx, resolvedPolicyName, err := resolveFirewallPolicyIdx(page, ref)
		if err != nil {
			return err
		}
		idx = resolvedIdx
		resolvedName = resolvedPolicyName
	}

	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabPageURL, url.QueryEscape(idx))
	if _, err := page.Goto(tabPageURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("방화벽 정책 상세 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "방화벽 정책 상세 페이지 접속"); err != nil {
		return err
	}

	currentTargets, _, err := fetchFirewallInternationalTargets(page, idx)
	if err != nil {
		return err
	}
	if len(currentTargets) == 0 {
		fmt.Printf("ℹ️ [%s | IDX:%s] 국제망 통신 정책이 비어 있어 개별 제거를 건너뜁니다.\n", strings.TrimSpace(resolvedName), idx)
		return nil
	}

	removeSet := toUpperSet(removeTargets)
	remaining := make([]string, 0, len(currentTargets))
	removed := make([]string, 0, len(removeTargets))
	for _, target := range currentTargets {
		upper := strings.ToUpper(strings.TrimSpace(target))
		if removeSet[upper] {
			removed = append(removed, upper)
			continue
		}
		remaining = append(remaining, upper)
	}

	if len(removed) == 0 {
		fmt.Printf("ℹ️ [%s | IDX:%s] 요청한 국제망 대상이 현재 정책에 없어 개별 제거를 건너뜁니다: %s\n", strings.TrimSpace(resolvedName), idx, strings.Join(removeTargets, ","))
		return nil
	}

	firewallDebugf(debug, "international remove | idx=%s current=%v remove=%v remain=%v", idx, currentTargets, removeTargets, remaining)
	return runUpdateFirewallInternational(page, idx, remaining, debug, "개별 제거")
}

func RunClearFirewallInternational(page playwright.Page, policyRef string, debug bool) error {
	return runUpdateFirewallInternational(page, policyRef, []string{}, debug, "전체 제거")
}

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

	ref := strings.TrimSpace(policyRef)
	idx := ref
	resolvedName := ref
	if !numericIDRegex.MatchString(ref) {
		if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
		}
		if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
			return err
		}

		resolvedIdx, resolvedPolicyName, err := resolveFirewallPolicyIdx(page, ref)
		if err != nil {
			return err
		}
		idx = resolvedIdx
		resolvedName = resolvedPolicyName
	}

	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabPageURL, url.QueryEscape(idx))
	if _, err := page.Goto(tabPageURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("방화벽 정책 상세 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "방화벽 정책 상세 페이지 접속"); err != nil {
		return err
	}

	currentTargets, _, err := fetchFirewallBotTargets(page, idx)
	if err != nil {
		return err
	}
	if len(currentTargets) == 0 {
		fmt.Printf("ℹ️ [%s | IDX:%s] 검색봇 정책이 비어 있어 개별 제거를 건너뜁니다.\n", strings.TrimSpace(resolvedName), idx)
		return nil
	}

	removeSet := toUpperSet(removeTargets)
	remaining := make([]string, 0, len(currentTargets))
	removed := make([]string, 0, len(removeTargets))
	for _, target := range currentTargets {
		upper := strings.ToUpper(strings.TrimSpace(target))
		if removeSet[upper] {
			removed = append(removed, upper)
			continue
		}
		remaining = append(remaining, upper)
	}

	if len(removed) == 0 {
		fmt.Printf("ℹ️ [%s | IDX:%s] 요청한 검색봇 대상이 현재 정책에 없어 개별 제거를 건너뜁니다: %s\n", strings.TrimSpace(resolvedName), idx, strings.Join(removeTargets, ","))
		return nil
	}

	firewallDebugf(debug, "bot remove | idx=%s current=%v remove=%v remain=%v", idx, currentTargets, removeTargets, remaining)
	return runUpdateFirewallBot(page, idx, remaining, debug, "개별 제거")
}

func RunClearFirewallBot(page playwright.Page, policyRef string, debug bool) error {
	return runUpdateFirewallBot(page, policyRef, []string{}, debug, "전체 제거")
}

func runUpdateFirewallBot(page playwright.Page, policyRef string, targets []string, debug bool, actionLabel string) error {
	ref := strings.TrimSpace(policyRef)
	idx := ref
	resolvedName := ref
	if !numericIDRegex.MatchString(ref) {
		if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
		}
		if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
			return err
		}

		resolvedIdx, resolvedPolicyName, err := resolveFirewallPolicyIdx(page, ref)
		if err != nil {
			return err
		}
		idx = resolvedIdx
		resolvedName = resolvedPolicyName
		firewallDebugf(debug, "resolved policy | name=%s idx=%s", resolvedName, idx)
	}

	if numericIDRegex.MatchString(strings.TrimSpace(resolvedName)) {
		if nameByIdx, nameErr := resolveFirewallPolicyNameByIdx(page, idx); nameErr == nil && strings.TrimSpace(nameByIdx) != "" {
			resolvedName = strings.TrimSpace(nameByIdx)
			firewallDebugf(debug, "resolved policy name by idx | idx=%s name=%s", idx, resolvedName)
		} else if nameErr != nil {
			firewallDebugf(debug, "resolve policy name by idx warning | idx=%s err=%v", idx, nameErr)
		}
	}

	titleFallback := strings.TrimSpace(resolvedName)
	if strings.TrimSpace(titleFallback) == strings.TrimSpace(idx) {
		titleFallback = ""
	}

	targetText := strings.Join(targets, ",")
	if len(targets) == 0 {
		targetText = "(empty)"
	}
	fmt.Printf("🚀 ELCAP 검색봇 접근 정책을 %s 중입니다... (%s)\n", actionLabel, targetText)
	firewallDebugf(debug, "bot set start | action=%s ref=%s idx=%s targets=%v", actionLabel, policyRef, idx, targets)
	firewallDebugf(debug, "bot engine=edit-form-v1")

	editPageURL := fmt.Sprintf("%s/%s/edit?tab=bot", firewallURL, url.PathEscape(idx))
	firewallDebugf(debug, "goto edit page | url=%s", editPageURL)
	if _, err := page.Goto(editPageURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("방화벽 정책 편집 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "방화벽 정책 편집 페이지 접속"); err != nil {
		return err
	}
	if err := waitForFirewallWriteFormReady(page, 8*time.Second); err != nil {
		return err
	}

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
	if strings.TrimSpace(formCounts.Title) == "" && titleFallback == "" {
		return fmt.Errorf("검색봇 정책 저장 중단: 현재 폼에 정책명(title)이 비어 있고 대체 정책명도 찾지 못했습니다")
	}

	actualTargets, rows, precheckErr := fetchFirewallBotTargets(page, idx)
	if precheckErr == nil {
		if equalFirewallBotTargets(targets, actualTargets) {
			fmt.Printf("ℹ️ [%s | IDX:%s] 검색봇 접근 정책이 이미 동일하여 저장을 건너뜁니다: %s\n", strings.TrimSpace(resolvedName), idx, strings.Join(actualTargets, ","))
			firewallDebugf(debug, "bot precheck same targets | %v", actualTargets)
			return nil
		}
		firewallDebugf(debug, "bot precheck mismatch actual=%v expected=%v rows=%d", actualTargets, targets, len(rows))
	} else {
		firewallDebugf(debug, "bot precheck error | %v", precheckErr)
	}

	_, submitDiag, err := submitFirewallBotTargets(page, idx, targets, titleFallback)
	if err != nil {
		firewallDebugf(debug, "bot submit failed | %v", err)
		logFirewallSubmitDiagnostics(debug, submitDiag)
		return fmt.Errorf("검색봇 접근 정책 저장 실패: %w", err)
	}
	logFirewallSubmitDiagnostics(debug, submitDiag)

	ok, verifyErr := waitForFirewallBotApplied(page, idx, targets, 22*time.Second, debug)
	if ok {
		fmt.Printf("✅ [%s | IDX:%s] 검색봇 접근 정책 %s 완료: %s\n", strings.TrimSpace(resolvedName), idx, actionLabel, targetText)
		return nil
	}
	if verifyErr != nil {
		return fmt.Errorf("검색봇 접근 정책 저장 후 검증 실패: %w", verifyErr)
	}
	return fmt.Errorf("검색봇 접근 정책 저장 후 반영을 확인하지 못했습니다. --firewall-tab bot --firewall-ref \"%s\"로 재확인하세요", idx)
}

func runUpdateFirewallInternational(page playwright.Page, policyRef string, targets []string, debug bool, actionLabel string) error {
	ref := strings.TrimSpace(policyRef)
	idx := ref
	resolvedName := ref
	if !numericIDRegex.MatchString(ref) {
		if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
		}
		if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
			return err
		}

		resolvedIdx, resolvedPolicyName, err := resolveFirewallPolicyIdx(page, ref)
		if err != nil {
			return err
		}
		idx = resolvedIdx
		resolvedName = resolvedPolicyName
		firewallDebugf(debug, "resolved policy | name=%s idx=%s", resolvedName, idx)
	}

	if numericIDRegex.MatchString(strings.TrimSpace(resolvedName)) {
		if nameByIdx, nameErr := resolveFirewallPolicyNameByIdx(page, idx); nameErr == nil && strings.TrimSpace(nameByIdx) != "" {
			resolvedName = strings.TrimSpace(nameByIdx)
			firewallDebugf(debug, "resolved policy name by idx | idx=%s name=%s", idx, resolvedName)
		} else if nameErr != nil {
			firewallDebugf(debug, "resolve policy name by idx warning | idx=%s err=%v", idx, nameErr)
		}
	}

	titleFallback := strings.TrimSpace(resolvedName)
	if strings.TrimSpace(titleFallback) == strings.TrimSpace(idx) {
		titleFallback = ""
	}

	targetText := strings.Join(targets, ",")
	if len(targets) == 0 {
		targetText = "(empty)"
	}
	fmt.Printf("🚀 ELCAP 국제망 통신 정책을 %s 중입니다... (%s)\n", actionLabel, targetText)
	firewallDebugf(debug, "international set start | action=%s ref=%s idx=%s targets=%v", actionLabel, policyRef, idx, targets)
	firewallDebugf(debug, "international engine=edit-form-v4")

	editPageURL := fmt.Sprintf("%s/%s/edit?tab=international", firewallURL, url.PathEscape(idx))
	firewallDebugf(debug, "goto edit page | url=%s", editPageURL)
	if _, err := page.Goto(editPageURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("방화벽 정책 편집 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "방화벽 정책 편집 페이지 접속"); err != nil {
		return err
	}
	if err := waitForFirewallWriteFormReady(page, 8*time.Second); err != nil {
		return err
	}

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

	inRows, inErr := fetchFirewallTabRows(page, "inbound", idx)
	outRows, outErr := fetchFirewallTabRows(page, "outbound", idx)
	if inErr == nil && outErr == nil {
		inData := estimateFirewallRuleDataRows(inRows)
		outData := estimateFirewallRuleDataRows(outRows)
		if strings.TrimSpace(formCounts.Title) == "" {
			if titleFallback == "" {
				return fmt.Errorf("국제망 정책 저장 중단: 현재 폼에 정책명(title)이 비어 있고 대체 정책명도 찾지 못했습니다")
			}
			firewallDebugf(debug, "international form title empty; fallback title=%q will be used", titleFallback)
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

	actualTargets, rows, precheckErr := fetchFirewallInternationalTargets(page, idx)
	if precheckErr == nil {
		if equalInternationalTargets(targets, actualTargets) {
			fmt.Printf("ℹ️ [%s | IDX:%s] 국제망 통신 정책이 이미 동일하여 저장을 건너뜁니다: %s\n", strings.TrimSpace(resolvedName), idx, strings.Join(actualTargets, ","))
			firewallDebugf(debug, "international precheck same targets | %v", actualTargets)
			return nil
		}
		firewallDebugf(debug, "international precheck mismatch actual=%v expected=%v rows=%d", actualTargets, targets, len(rows))
	} else {
		firewallDebugf(debug, "international precheck error | %v", precheckErr)
	}

	_, submitDiag, err := submitFirewallInternationalTargets(page, idx, targets, titleFallback)
	if err != nil && len(targets) > 0 && strings.Contains(err.Error(), "기존 국제망 정책의 idx를 찾지 못해 안전하게 덮어쓸 수 없습니다") {
		firewallDebugf(debug, "international submit fallback(two-phase) | reason=%v", err)
		logFirewallSubmitDiagnostics(debug, submitDiag)

		// 국제망 idx를 못 찾는 UI 케이스가 있어, 먼저 국제망만 비운 뒤 원하는 값을 다시 설정한다.
		_, clearDiag, clearErr := submitFirewallInternationalTargets(page, idx, []string{}, titleFallback)
		if clearErr != nil {
			firewallDebugf(debug, "international clear phase failed | %v", clearErr)
			logFirewallSubmitDiagnostics(debug, clearDiag)
			return fmt.Errorf("국제망 통신 정책 저장 실패(1차 clear 단계): %w", clearErr)
		}
		logFirewallSubmitDiagnostics(debug, clearDiag)
		page.WaitForTimeout(500)

		_, submitDiag, err = submitFirewallInternationalTargets(page, idx, targets, titleFallback)
	}
	if err != nil {
		firewallDebugf(debug, "international submit failed | %v", err)
		logFirewallSubmitDiagnostics(debug, submitDiag)
		return fmt.Errorf("국제망 통신 정책 저장 실패: %w", err)
	}
	logFirewallSubmitDiagnostics(debug, submitDiag)

	ok, verifyErr := waitForFirewallInternationalApplied(page, idx, targets, 22*time.Second, debug)
	if ok {
		fmt.Printf("✅ [%s | IDX:%s] 국제망 통신 정책 %s 완료: %s\n", strings.TrimSpace(resolvedName), idx, actionLabel, targetText)
		return nil
	}
	if verifyErr != nil {
		return fmt.Errorf("국제망 통신 정책 저장 후 검증 실패: %w", verifyErr)
	}
	return fmt.Errorf("국제망 통신 정책 저장 후 반영을 확인하지 못했습니다. --firewall-tab international --firewall-ref \"%s\"로 재확인하세요", idx)
}

func normalizeFirewallTab(tabInput string) (string, string, error) {
	tab := strings.ToLower(strings.TrimSpace(tabInput))
	switch tab {
	case "inbound", "인바운드":
		return "inbound", "인바운드", nil
	case "outbound", "아웃바운드":
		return "outbound", "아웃바운드", nil
	case "international", "국제망", "국제망통신":
		return "international", "국제망 통신", nil
	case "bot", "봇":
		return "bot", "봇", nil
	default:
		return "", "", fmt.Errorf("지원하지 않는 --firewall-tab 값입니다: %q (가능값: inbound|outbound|international|bot)", tabInput)
	}
}

func normalizeFirewallDirection(input string) (tab string, bound string, label string, err error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "inbound", "in", "인바운드":
		return "inbound", "IN", "인바운드", nil
	case "outbound", "out", "아웃바운드":
		return "outbound", "OUT", "아웃바운드", nil
	default:
		return "", "", "", fmt.Errorf("지원하지 않는 --firewall-dir 값입니다: %q (가능값: inbound|outbound|both)", input)
	}
}

func normalizeFirewallDirections(input string) ([]firewallDirectionTarget, error) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	if normalized == "both" || normalized == "all" || normalized == "양방향" || normalized == "inout" {
		return []firewallDirectionTarget{
			{Tab: "inbound", Bound: "IN", Label: "인바운드"},
			{Tab: "outbound", Bound: "OUT", Label: "아웃바운드"},
		}, nil
	}

	parts := strings.Split(input, ",")
	result := make([]firewallDirectionTarget, 0, len(parts))
	seen := map[string]bool{}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tab, bound, label, err := normalizeFirewallDirection(part)
		if err != nil {
			return nil, fmt.Errorf("지원하지 않는 --firewall-dir 값입니다: %q (가능값: inbound|outbound|both)", input)
		}
		if seen[tab] {
			continue
		}
		seen[tab] = true
		result = append(result, firewallDirectionTarget{
			Tab:   tab,
			Bound: bound,
			Label: label,
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("지원하지 않는 --firewall-dir 값입니다: %q (가능값: inbound|outbound|both)", input)
	}

	return result, nil
}

func normalizeFirewallInternationalTargets(input string) ([]string, error) {
	return parseFirewallInternationalTargetList(input, "--firewall-international", true)
}

func normalizeFirewallInternationalRemoveTargets(input string) ([]string, error) {
	return parseFirewallInternationalTargetList(input, "--firewall-international-remove", false)
}

func normalizeFirewallBotTargets(input string) ([]string, error) {
	return parseFirewallBotTargetList(input, "--firewall-bot", true)
}

func normalizeFirewallBotRemoveTargets(input string) ([]string, error) {
	return parseFirewallBotTargetList(input, "--firewall-bot-remove", false)
}

func parseFirewallInternationalTargetList(input, optionName string, enforceForeignExclusive bool) ([]string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return nil, fmt.Errorf("%s 값이 비어 있습니다", optionName)
	}

	replacer := strings.NewReplacer(";", ",", "|", ",", "/", ",", "\n", ",", "\t", ",")
	raw = replacer.Replace(raw)
	parts := strings.Split(raw, ",")
	if len(parts) == 1 {
		parts = strings.Fields(raw)
	}

	result := make([]string, 0, len(parts))
	seen := map[string]bool{}
	invalid := make([]string, 0)
	for _, part := range parts {
		token := strings.ToUpper(strings.TrimSpace(part))
		if token == "" {
			continue
		}
		normalized, ok := firewallInternationalAliasMap[token]
		if !ok {
			invalid = append(invalid, part)
			continue
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf(
			"지원하지 않는 %s 값이 있습니다: %s (가능값: %s)",
			optionName,
			strings.Join(invalid, ", "),
			strings.Join(firewallInternationalAllowed, ","),
		)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s 값이 비어 있습니다 (가능값: %s)", optionName, strings.Join(firewallInternationalAllowed, ","))
	}

	if enforceForeignExclusive && seen["FOREIGN"] && len(result) > 1 {
		return nil, fmt.Errorf("FOREIGN은 단독으로만 사용할 수 있습니다 (예: %s FOREIGN)", optionName)
	}

	return result, nil
}

func parseFirewallBotTargetList(input, optionName string, enforceAllExclusive bool) ([]string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return nil, fmt.Errorf("%s 값이 비어 있습니다", optionName)
	}

	replacer := strings.NewReplacer(";", ",", "|", ",", "/", ",", "\n", ",", "\t", ",")
	raw = replacer.Replace(raw)
	parts := strings.Split(raw, ",")
	if len(parts) == 1 {
		parts = strings.Fields(raw)
	}

	result := make([]string, 0, len(parts))
	seen := map[string]bool{}
	invalid := make([]string, 0)
	for _, part := range parts {
		token := strings.ToUpper(strings.TrimSpace(part))
		if token == "" {
			continue
		}
		normalized, ok := firewallBotAliasMap[token]
		if !ok {
			invalid = append(invalid, part)
			continue
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf(
			"지원하지 않는 %s 값이 있습니다: %s (가능값: %s)",
			optionName,
			strings.Join(invalid, ", "),
			strings.Join(firewallBotAllowed, ","),
		)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s 값이 비어 있습니다 (가능값: %s)", optionName, strings.Join(firewallBotAllowed, ","))
	}
	if enforceAllExclusive && seen["ALL"] && len(result) > 1 {
		return nil, fmt.Errorf("ALL은 단독으로만 사용할 수 있습니다 (예: %s ALL)", optionName)
	}

	return result, nil
}

func normalizeFirewallProtocol(input string) (string, error) {
	p := strings.ToUpper(strings.TrimSpace(input))
	switch p {
	case "TCP", "UDP":
		return p, nil
	default:
		return "", fmt.Errorf("지원하지 않는 --rule-protocol 값입니다: %q (가능값: TCP|UDP)", input)
	}
}

func resolveFirewallPolicyIdx(page playwright.Page, policyRef string) (string, string, error) {
	ref := strings.TrimSpace(policyRef)
	if ref == "" {
		return "", "", fmt.Errorf("방화벽 정책 식별자가 비어 있습니다")
	}

	if numericIDRegex.MatchString(ref) {
		return ref, ref, nil
	}

	if err := waitForXPathVisible(page, "ELCAP 정책 테이블", firewallTable2XPath, 8*time.Second); err != nil {
		return "", "", err
	}

	policies, err := getFirewallPolicies(page)
	if err != nil {
		return "", "", err
	}

	matches := make([]FirewallPolicy, 0, 4)
	for _, p := range policies {
		if strings.Contains(p.Name, ref) {
			matches = append(matches, p)
		}
	}

	if len(matches) == 0 {
		return "", "", fmt.Errorf("ELCAP 정책 '%s'을(를) 찾을 수 없습니다. 먼저 --firewall-list로 IDX를 확인하세요", ref)
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%s(IDX:%s)", m.Name, m.Idx))
		}
		return "", "", fmt.Errorf("ELCAP 정책 '%s'이(가) 여러 개입니다: %s", ref, strings.Join(names, " | "))
	}

	return matches[0].Idx, matches[0].Name, nil
}

func resolveFirewallPolicyNameByIdx(page playwright.Page, idx string) (string, error) {
	idx = strings.TrimSpace(idx)
	if idx == "" {
		return "", fmt.Errorf("정책 IDX가 비어 있습니다")
	}

	if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return "", fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
		return "", err
	}
	if err := waitForXPathVisible(page, "ELCAP 정책 테이블", firewallTable2XPath, 8*time.Second); err != nil {
		return "", err
	}

	policies, err := getFirewallPolicies(page)
	if err != nil {
		return "", err
	}
	for _, policy := range policies {
		if strings.TrimSpace(policy.Idx) != idx {
			continue
		}
		name := strings.TrimSpace(policy.Name)
		if name != "" {
			return name, nil
		}
	}
	return "", fmt.Errorf("IDX %s에 해당하는 정책명을 찾지 못했습니다", idx)
}

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
		const looksLikeLogin = lower.includes("input[name='id']") || lower.includes("name=\"id\"") && lower.includes("name=\"pw\"");
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
		const looksLikeLogin = lower.includes("input[name='id']") || lower.includes("name=\"id\"") && lower.includes("name=\"pw\"");
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
		const looksLikeLogin = lower.includes("input[name='id']") || lower.includes("name=\"id\"") && lower.includes("name=\"pw\"");
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
		const looksLikeLogin = lower.includes("input[name='id']") || lower.includes("name=\"id\"") && lower.includes("name=\"pw\"");
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

func waitForFirewallWriteFormReady(page playwright.Page, timeout time.Duration) error {
	return waitForCondition(page, timeout, func() (bool, error) {
		raw, err := page.Evaluate(`() => {
			const form =
				document.querySelector("form[name='modal']") ||
				document.querySelector("form[action='/firewall']") ||
				document.querySelector("form[action='https://console.iwinv.kr/firewall']") ||
				document.querySelector("form[action*='/firewall']") ||
				document.querySelector("main form");
			if (!form) return false;

			const token = form.querySelector("input[name='_token']") || document.querySelector('meta[name="csrf-token"]');
			const idx = form.querySelector("input[name='idx']") || document.querySelector("input[name='idx']");
			return !!token && !!idx;
		}`)
		if err != nil {
			return false, err
		}

		ready, ok := raw.(bool)
		if !ok {
			return false, fmt.Errorf("폼 준비 상태를 해석할 수 없습니다")
		}
		return ready, nil
	})
}

func getFirewallFormSectionCounts(page playwright.Page) (firewallFormSectionCounts, error) {
	raw, err := page.Evaluate(`() => {
		const form =
			document.querySelector("form[name='modal']") ||
			document.querySelector("form[action='/firewall']") ||
			document.querySelector("form[action='https://console.iwinv.kr/firewall']") ||
			document.querySelector("form[action*='/firewall']") ||
			document.querySelector("main form");
		if (!form) {
			return { ok: false, reason: "form not found" };
		}

		const getCount = (name) => {
			return Array.from(document.querySelectorAll("[name='" + name.replace(/'/g, "\\'") + "']"))
				.filter((el) => !el.form || el.form === form)
				.length;
		};
		const titleEl = (form.querySelector("[name='title']") || document.querySelector("[name='title']"));
		const title = titleEl ? String(titleEl.value || "").trim() : "";

		return {
			ok: true,
			title,
			inboundCount: getCount("inbound[idx][]"),
			outboundCount: getCount("outbound[idx][]"),
			internationalCount: getCount("international[idx][]"),
			botCount: getCount("bot[idx][]")
		};
	}`)
	if err != nil {
		return firewallFormSectionCounts{}, err
	}

	obj, ok := raw.(map[string]interface{})
	if !ok {
		return firewallFormSectionCounts{}, fmt.Errorf("폼 카운트 응답 형식을 해석할 수 없습니다")
	}
	okVal, _ := obj["ok"].(bool)
	if !okVal {
		reason, _ := obj["reason"].(string)
		if reason == "" {
			reason = "unknown"
		}
		return firewallFormSectionCounts{}, fmt.Errorf("%s", reason)
	}

	return firewallFormSectionCounts{
		Title:              strings.TrimSpace(toString(obj["title"])),
		InboundCount:       toInt(obj["inboundCount"]),
		OutboundCount:      toInt(obj["outboundCount"]),
		InternationalCount: toInt(obj["internationalCount"]),
		BotCount:           toInt(obj["botCount"]),
	}, nil
}

func waitForFirewallRuleApplied(page playwright.Page, idx, tab, protocol, port, ruleIP string, timeout time.Duration, debug bool) (bool, error) {
	deadline := time.Now().Add(timeout)
	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabPageURL, url.QueryEscape(idx))
	var lastErr error
	attempt := 0

	for {
		attempt++
		if _, err := page.Goto(tabPageURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			lastErr = err
			firewallDebugf(debug, "verify attempt=%d goto error=%v", attempt, err)
		} else if err := ensureAuthenticated(page, "방화벽 정책 상세 페이지 접속(검증)"); err != nil {
			lastErr = err
			firewallDebugf(debug, "verify attempt=%d auth error=%v", attempt, err)
		} else {
			rows, err := fetchFirewallTabRows(page, tab, idx)
			if err == nil && hasRuleRow(rows, protocol, port, ruleIP) {
				firewallDebugf(debug, "verify attempt=%d matched in tab rows (%d rows)", attempt, len(rows))
				return true, nil
			}
			if err != nil {
				lastErr = err
				firewallDebugf(debug, "verify attempt=%d tab fetch error=%v", attempt, err)
			} else {
				firewallDebugf(debug, "verify attempt=%d no match in tab rows (%d rows)", attempt, len(rows))
			}

			ok, err := hasRuleUniqueInForm(page, tab, protocol, port, ruleIP)
			if err == nil && ok {
				firewallDebugf(debug, "verify attempt=%d matched in form unique", attempt)
				return true, nil
			}
			if err != nil {
				lastErr = err
				firewallDebugf(debug, "verify attempt=%d form unique check error=%v", attempt, err)
			} else {
				firewallDebugf(debug, "verify attempt=%d form unique not found", attempt)
			}
		}

		if time.Now().After(deadline) {
			break
		}
		page.WaitForTimeout(600)
	}

	return false, lastErr
}

func waitForFirewallRuleRemoved(page playwright.Page, idx, tab, protocol, port, ruleIP string, timeout time.Duration, debug bool) (bool, error) {
	deadline := time.Now().Add(timeout)
	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabPageURL, url.QueryEscape(idx))
	var lastErr error
	attempt := 0

	for {
		attempt++
		if _, err := page.Goto(tabPageURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			lastErr = err
			firewallDebugf(debug, "remove verify attempt=%d goto error=%v", attempt, err)
		} else if err := ensureAuthenticated(page, "방화벽 정책 상세 페이지 접속(삭제 검증)"); err != nil {
			lastErr = err
			firewallDebugf(debug, "remove verify attempt=%d auth error=%v", attempt, err)
		} else {
			rows, err := fetchFirewallTabRows(page, tab, idx)
			matchedInRows := false
			if err == nil {
				matchedInRows = hasRuleRow(rows, protocol, port, ruleIP)
			}
			if err != nil {
				lastErr = err
				firewallDebugf(debug, "remove verify attempt=%d tab fetch error=%v", attempt, err)
			}

			matchedInForm := false
			formFound, formErr := hasRuleUniqueInForm(page, tab, protocol, port, ruleIP)
			if formErr == nil {
				matchedInForm = formFound
			} else {
				lastErr = formErr
				firewallDebugf(debug, "remove verify attempt=%d form unique check error=%v", attempt, formErr)
			}

			if !matchedInRows && !matchedInForm && err == nil && formErr == nil {
				firewallDebugf(debug, "remove verify attempt=%d no match in rows/form", attempt)
				return true, nil
			}
			firewallDebugf(debug, "remove verify attempt=%d still present rows=%t form=%t", attempt, matchedInRows, matchedInForm)
		}

		if time.Now().After(deadline) {
			break
		}
		page.WaitForTimeout(600)
	}

	return false, lastErr
}

func waitForFirewallInternationalApplied(page playwright.Page, idx string, targets []string, timeout time.Duration, debug bool) (bool, error) {
	deadline := time.Now().Add(timeout)
	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabPageURL, url.QueryEscape(idx))
	var lastErr error
	attempt := 0
	expectedTargets := normalizeFirewallInternationalTargetValues(targets)

	for {
		attempt++
		if _, err := page.Goto(tabPageURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			lastErr = err
			firewallDebugf(debug, "international verify attempt=%d goto error=%v", attempt, err)
		} else if err := ensureAuthenticated(page, "방화벽 정책 상세 페이지 접속(국제망 검증)"); err != nil {
			lastErr = err
			firewallDebugf(debug, "international verify attempt=%d auth error=%v", attempt, err)
		} else {
			actualTargets, rows, err := fetchFirewallInternationalTargets(page, idx)
			if err == nil && equalInternationalTargets(expectedTargets, actualTargets) {
				firewallDebugf(debug, "international verify attempt=%d matched targets=%v rows=%d", attempt, actualTargets, len(rows))
				return true, nil
			}
			if err != nil {
				lastErr = err
				firewallDebugf(debug, "international verify attempt=%d target fetch error=%v", attempt, err)
			} else {
				firewallDebugf(debug, "international verify attempt=%d mismatch actual=%v expected=%v rows=%d", attempt, actualTargets, expectedTargets, len(rows))
			}

			// fallback: 현재 문서에 hidden 필드가 직접 노출되는 경우도 있어 추가 확인한다.
			matched, actualInForm, formErr := hasInternationalTargetsInForm(page, expectedTargets)
			if formErr == nil && matched {
				firewallDebugf(debug, "international verify attempt=%d matched in form targets=%v", attempt, actualInForm)
				return true, nil
			}
			if formErr != nil {
				lastErr = formErr
				firewallDebugf(debug, "international verify attempt=%d form fallback error=%v", attempt, formErr)
			}
		}

		if time.Now().After(deadline) {
			break
		}
		page.WaitForTimeout(600)
	}

	return false, lastErr
}

func waitForFirewallBotApplied(page playwright.Page, idx string, targets []string, timeout time.Duration, debug bool) (bool, error) {
	deadline := time.Now().Add(timeout)
	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabPageURL, url.QueryEscape(idx))
	var lastErr error
	attempt := 0
	expectedTargets := normalizeFirewallBotTargetValues(targets)

	for {
		attempt++
		if _, err := page.Goto(tabPageURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			lastErr = err
			firewallDebugf(debug, "bot verify attempt=%d goto error=%v", attempt, err)
		} else if err := ensureAuthenticated(page, "방화벽 정책 상세 페이지 접속(검색봇 검증)"); err != nil {
			lastErr = err
			firewallDebugf(debug, "bot verify attempt=%d auth error=%v", attempt, err)
		} else {
			actualTargets, rows, err := fetchFirewallBotTargets(page, idx)
			// 봇 탭이 일시적으로 빈 응답(rows=0)을 줄 수 있어, empty 매칭은 rows 신뢰도가 확보될 때만 인정한다.
			if err == nil && equalFirewallBotTargets(expectedTargets, actualTargets) {
				if len(expectedTargets) == 0 && len(actualTargets) == 0 && len(rows) == 0 {
					firewallDebugf(debug, "bot verify attempt=%d ambiguous empty response; retry", attempt)
					goto formFallback
				}
				firewallDebugf(debug, "bot verify attempt=%d matched targets=%v rows=%d", attempt, actualTargets, len(rows))
				return true, nil
			}
			if err != nil {
				lastErr = err
				firewallDebugf(debug, "bot verify attempt=%d target fetch error=%v", attempt, err)
			} else {
				firewallDebugf(debug, "bot verify attempt=%d mismatch actual=%v expected=%v rows=%d", attempt, actualTargets, expectedTargets, len(rows))
			}

		formFallback:
			matched, actualInForm, formErr := hasBotTargetsInForm(page, expectedTargets)
			if formErr == nil && matched {
				firewallDebugf(debug, "bot verify attempt=%d matched in form targets=%v", attempt, actualInForm)
				return true, nil
			}
			if formErr != nil {
				lastErr = formErr
				firewallDebugf(debug, "bot verify attempt=%d form fallback error=%v", attempt, formErr)
			}
		}

		if time.Now().After(deadline) {
			break
		}
		page.WaitForTimeout(600)
	}

	return false, lastErr
}

func hasBotTargetsInForm(page playwright.Page, expectedTargets []string) (bool, []string, error) {
	raw, err := page.Evaluate(`([expectedTargets]) => {
		const normalize = (v) => String(v || "").trim().toUpperCase();
		const dedupe = (arr) => {
			const out = [];
			const seen = new Set();
			for (const v of arr) {
				const n = normalize(v);
				if (!n || seen.has(n)) continue;
				seen.add(n);
				out.push(n);
			}
			return out;
		};

		const expected = dedupe(Array.isArray(expectedTargets) ? expectedTargets : []);
		const targetInputs = Array.from(document.querySelectorAll("[name='bot[target][]']"));
		const actual = dedupe(targetInputs.map((el) => el.value));

		const expectedSet = new Set(expected);
		const actualSet = new Set(actual);
		let match = expectedSet.size === actualSet.size;
		if (match) {
			for (const v of expectedSet) {
				if (!actualSet.has(v)) {
					match = false;
					break;
				}
			}
		}
		return { match, actual, targetInputCount: targetInputs.length };
	}`, []interface{}{expectedTargets})
	if err != nil {
		return false, nil, err
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("검색봇 폼 검증 응답 형식을 해석할 수 없습니다")
	}
	targetInputCount := toInt(res["targetInputCount"])
	if targetInputCount == 0 {
		return false, nil, nil
	}

	match, _ := res["match"].(bool)
	actual := stringifyJSArray(res["actual"])
	return match, actual, nil
}

func hasInternationalTargetsInForm(page playwright.Page, expectedTargets []string) (bool, []string, error) {
	raw, err := page.Evaluate(`([expectedTargets]) => {
		const normalize = (v) => String(v || "").trim().toUpperCase();
		const dedupe = (arr) => {
			const out = [];
			const seen = new Set();
			for (const v of arr) {
				const n = normalize(v);
				if (!n || seen.has(n)) continue;
				seen.add(n);
				out.push(n);
			}
			return out;
		};

		const expected = dedupe(Array.isArray(expectedTargets) ? expectedTargets : []);
		const targetInputs = Array.from(document.querySelectorAll("[name='international[target][]']"));
		const actual = dedupe(
			targetInputs.map((el) => el.value)
		);

		const expectedSet = new Set(expected);
		const actualSet = new Set(actual);
		let match = expectedSet.size === actualSet.size;
		if (match) {
			for (const v of expectedSet) {
				if (!actualSet.has(v)) {
					match = false;
					break;
				}
			}
		}

		return { match, actual, targetInputCount: targetInputs.length };
	}`, []interface{}{expectedTargets})
	if err != nil {
		return false, nil, err
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("국제망 폼 검증 응답 형식을 해석할 수 없습니다")
	}

	targetInputCount := toInt(res["targetInputCount"])
	if targetInputCount == 0 {
		return false, nil, nil
	}

	match, _ := res["match"].(bool)
	actual := stringifyJSArray(res["actual"])
	return match, actual, nil
}

func hasInternationalRows(rows [][]string, targets []string) bool {
	if len(targets) == 0 {
		return false
	}

	joined := strings.ToUpper(strings.Join(flattenRows(rows), " "))
	if joined == "" {
		return false
	}

	expected := toUpperSet(targets)
	actual := map[string]bool{}
	for _, code := range firewallInternationalAllowed {
		hints, ok := firewallInternationalDisplayHints[code]
		if !ok || len(hints) == 0 {
			hints = []string{code}
		}
		for _, hint := range hints {
			hint = strings.ToUpper(strings.TrimSpace(hint))
			if hint == "" {
				continue
			}
			if strings.Contains(joined, hint) {
				actual[code] = true
				break
			}
		}
	}

	// 국제망 정책은 FOREIGN(한국만 허용)과 개별 국가 차단이 공존하면 안 된다.
	if actual["FOREIGN"] && len(actual) > 1 {
		return false
	}
	if expected["FOREIGN"] && len(expected) > 1 {
		return false
	}

	return equalStringSet(expected, actual)
}

func flattenRows(rows [][]string) []string {
	flat := make([]string, 0, len(rows)*2)
	for _, row := range rows {
		flat = append(flat, row...)
	}
	return flat
}

func toUpperSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, v := range values {
		key := strings.ToUpper(strings.TrimSpace(v))
		if key == "" {
			continue
		}
		set[key] = true
	}
	return set
}

func equalStringSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func firewallDebugf(enabled bool, format string, args ...interface{}) {
	if !enabled {
		return
	}
	fmt.Printf("🧪 FWDEBUG | "+format+"\n", args...)
}

func logFirewallSubmitDiagnostics(enabled bool, diag map[string]interface{}) {
	if !enabled {
		return
	}
	if len(diag) == 0 {
		firewallDebugf(true, "submit diag empty")
		return
	}

	if formFound, ok := diag["formFound"].(bool); ok {
		firewallDebugf(true, "formFound=%t", formFound)
	}
	if action, ok := diag["formAction"].(string); ok && action != "" {
		firewallDebugf(true, "formAction=%s", action)
	}
	if route, ok := diag["formRoute"].(string); ok && route != "" {
		firewallDebugf(true, "formRoute=%s", route)
	}
	if currentURL, ok := diag["currentURL"].(string); ok && currentURL != "" {
		firewallDebugf(true, "currentURL=%s", currentURL)
	}
	if count, ok := diag["existingUniqueCount"].(float64); ok {
		firewallDebugf(true, "existingUniqueCount=%.0f", count)
	}
	if addUnique, ok := diag["addUnique"].(string); ok && addUnique != "" {
		firewallDebugf(true, "addUnique=%s", addUnique)
	}
	if addUniqueMask, ok := diag["addUniqueMask"].(string); ok && addUniqueMask != "" {
		firewallDebugf(true, "addUniqueMask=%s", addUniqueMask)
	}
	if removeUnique, ok := diag["removeUnique"].(string); ok && removeUnique != "" {
		firewallDebugf(true, "removeUnique=%s", removeUnique)
	}
	if removeUniqueMask, ok := diag["removeUniqueMask"].(string); ok && removeUniqueMask != "" {
		firewallDebugf(true, "removeUniqueMask=%s", removeUniqueMask)
	}
	if removeIndexes, ok := diag["removeIndexes"].([]interface{}); ok && len(removeIndexes) > 0 {
		firewallDebugf(true, "removeIndexes=%v", removeIndexes)
	}
	if removeMatchMode, ok := diag["removeMatchMode"].(string); ok && removeMatchMode != "" {
		firewallDebugf(true, "removeMatchMode=%s", removeMatchMode)
	}
	if removeRowCandidateCount, ok := diag["removeRowCandidateCount"].(float64); ok {
		firewallDebugf(true, "removeRowCandidateCount=%.0f", removeRowCandidateCount)
	}
	if removeTargetNormalized, ok := diag["removeTargetNormalized"].(string); ok && removeTargetNormalized != "" {
		firewallDebugf(true, "removeTargetNormalized=%s", removeTargetNormalized)
	}
	if totalRuleRows, ok := diag["totalRuleRows"].(float64); ok {
		firewallDebugf(true, "totalRuleRows=%.0f", totalRuleRows)
	}
	if removedCount, ok := diag["removedCount"].(float64); ok {
		firewallDebugf(true, "removedCount=%.0f", removedCount)
	}
	if targets, ok := diag["targets"].([]interface{}); ok && len(targets) > 0 {
		firewallDebugf(true, "internationalTargets=%v", targets)
	}
	if targets, ok := diag["botTargets"].([]interface{}); ok && len(targets) > 0 {
		firewallDebugf(true, "botTargets=%v", targets)
	}
	if before, ok := diag["beforeInternational"].([]interface{}); ok {
		firewallDebugf(true, "beforeInternational=%v", before)
	}
	if original, ok := diag["originalInternational"].([]interface{}); ok && len(original) > 0 {
		firewallDebugf(true, "originalInternational=%v", original)
	}
	if status, ok := diag["beforeInternationalFetchStatus"].(float64); ok {
		firewallDebugf(true, "beforeInternationalFetchStatus=%.0f", status)
	}
	if detected, ok := diag["beforeInternationalDetected"].([]interface{}); ok {
		firewallDebugf(true, "beforeInternationalDetected=%v", detected)
	}
	if preview, ok := diag["beforeInternationalFetchBodyPreview"].(string); ok && preview != "" {
		firewallDebugf(true, "beforeInternationalFetchBodyPreview=%q", preview)
	}
	if err, ok := diag["beforeInternationalFetchError"].(string); ok && err != "" {
		firewallDebugf(true, "beforeInternationalFetchError=%s", err)
	}
	if after, ok := diag["afterInternational"].([]interface{}); ok {
		firewallDebugf(true, "afterInternational=%v", after)
	}
	if submitted, ok := diag["submittedInternational"].([]interface{}); ok {
		firewallDebugf(true, "submittedInternational=%v", submitted)
	}
	if before, ok := diag["beforeBot"].([]interface{}); ok {
		firewallDebugf(true, "beforeBot=%v", before)
	}
	if status, ok := diag["beforeBotFetchStatus"].(float64); ok {
		firewallDebugf(true, "beforeBotFetchStatus=%.0f", status)
	}
	if detected, ok := diag["beforeBotDetected"].([]interface{}); ok {
		firewallDebugf(true, "beforeBotDetected=%v", detected)
	}
	if preview, ok := diag["beforeBotFetchBodyPreview"].(string); ok && preview != "" {
		firewallDebugf(true, "beforeBotFetchBodyPreview=%q", preview)
	}
	if err, ok := diag["beforeBotFetchError"].(string); ok && err != "" {
		firewallDebugf(true, "beforeBotFetchError=%s", err)
	}
	if after, ok := diag["afterBot"].([]interface{}); ok {
		firewallDebugf(true, "afterBot=%v", after)
	}
	if submitted, ok := diag["submittedBot"].([]interface{}); ok {
		firewallDebugf(true, "submittedBot=%v", submitted)
	}
	if filled, ok := diag["inboundUniqueFilled"].(float64); ok {
		firewallDebugf(true, "inboundUniqueFilled=%.0f", filled)
	}
	if filled, ok := diag["outboundUniqueFilled"].(float64); ok {
		firewallDebugf(true, "outboundUniqueFilled=%.0f", filled)
	}
	if count, ok := diag["inboundSubmitRows"].(float64); ok {
		firewallDebugf(true, "inboundSubmitRows=%.0f", count)
	}
	if count, ok := diag["outboundSubmitRows"].(float64); ok {
		firewallDebugf(true, "outboundSubmitRows=%.0f", count)
	}
	if count, ok := diag["botSubmitRows"].(float64); ok {
		firewallDebugf(true, "botSubmitRows=%.0f", count)
	}
	if count, ok := diag["internationalSubmitRows"].(float64); ok {
		firewallDebugf(true, "internationalSubmitRows=%.0f", count)
	}
	if count, ok := diag["submitParamCount"].(float64); ok {
		firewallDebugf(true, "submitParamCount=%.0f", count)
	}
	if count, ok := diag["inboundSubmitRows"].(float64); ok {
		firewallDebugf(true, "inboundSubmitRows=%.0f", count)
	}
	if count, ok := diag["outboundSubmitRows"].(float64); ok {
		firewallDebugf(true, "outboundSubmitRows=%.0f", count)
	}
	if preview, ok := diag["submitParamPreview"].([]interface{}); ok && len(preview) > 0 {
		max := len(preview)
		if max > 20 {
			max = 20
		}
		firewallDebugf(true, "submitParamPreview=%v", preview[:max])
	}
	if keyCounts, ok := diag["submitKeyCounts"].(map[string]interface{}); ok && len(keyCounts) > 0 {
		firewallDebugf(true, "submitKeyCounts=%v", keyCounts)
	}
	if removedRows, ok := diag["removedRows"].([]interface{}); ok && len(removedRows) > 0 {
		firewallDebugf(true, "removedRows=%v", removedRows)
	}
	if sectionFieldCount, ok := diag["sectionFieldCount"].(float64); ok {
		firewallDebugf(true, "sectionFieldCount=%.0f", sectionFieldCount)
	}
	if sectionFieldNames, ok := diag["sectionFieldNames"].([]interface{}); ok && len(sectionFieldNames) > 0 {
		firewallDebugf(true, "sectionFieldNames=%v", sectionFieldNames)
	}
	if sectionCountsBefore, ok := diag["sectionCountsBefore"].(map[string]interface{}); ok && len(sectionCountsBefore) > 0 {
		firewallDebugf(true, "sectionCountsBefore=%v", sectionCountsBefore)
	}
	if sectionCountsAfter, ok := diag["sectionCountsAfter"].(map[string]interface{}); ok && len(sectionCountsAfter) > 0 {
		firewallDebugf(true, "sectionCountsAfter=%v", sectionCountsAfter)
	}
	for _, prefix := range []string{"inbound", "outbound", "bot"} {
		if status, ok := diag[prefix+"HydrateStatus"].(float64); ok {
			firewallDebugf(true, "%sHydrateStatus=%.0f", prefix, status)
		}
		if count, ok := diag[prefix+"HydrateCount"].(float64); ok {
			firewallDebugf(true, "%sHydrateCount=%.0f", prefix, count)
		}
		if count, ok := diag[prefix+"HydrateRuleCount"].(float64); ok && count > 0 {
			firewallDebugf(true, "%sHydrateRuleCount=%.0f", prefix, count)
		}
		if count, ok := diag[prefix+"HydrateMissingIdxCount"].(float64); ok && count > 0 {
			firewallDebugf(true, "%sHydrateMissingIdxCount=%.0f", prefix, count)
		}
		if rows, ok := diag[prefix+"HydrateDataRows"].(float64); ok {
			firewallDebugf(true, "%sHydrateDataRows=%.0f", prefix, rows)
		}
		if byPanel, ok := diag[prefix+"HydrateByPanel"].(bool); ok && byPanel {
			firewallDebugf(true, "%sHydrateByPanel=true", prefix)
		}
		if count, ok := diag[prefix+"HydratePanelCount"].(float64); ok && count > 0 {
			firewallDebugf(true, "%sHydratePanelCount=%.0f", prefix, count)
		}
		if count, ok := diag[prefix+"HydratePanelScripts"].(float64); ok && count > 0 {
			firewallDebugf(true, "%sHydratePanelScripts=%.0f", prefix, count)
		}
		if skipped, ok := diag[prefix+"HydrateSkippedNoRows"].(bool); ok && skipped {
			firewallDebugf(true, "%sHydrateSkippedNoRows=true", prefix)
		}
		if preview, ok := diag[prefix+"HydrateBodyPreview"].(string); ok && preview != "" {
			firewallDebugf(true, "%sHydrateBodyPreview=%q", prefix, preview)
		}
	}
	if submitURL, ok := diag["submitURL"].(string); ok && submitURL != "" {
		firewallDebugf(true, "submitURL=%s", submitURL)
	}
	if status, ok := diag["status"].(float64); ok {
		firewallDebugf(true, "submitStatus=%.0f", status)
	}
	if bodyPreview, ok := diag["bodyPreview"].(string); ok && bodyPreview != "" {
		firewallDebugf(true, "submitBodyPreview=%q", bodyPreview)
	}
	if looksLikeLogin, ok := diag["looksLikeLogin"].(bool); ok {
		firewallDebugf(true, "looksLikeLogin=%t", looksLikeLogin)
	}
	if hasErrorKeyword, ok := diag["hasErrorKeyword"].(bool); ok {
		firewallDebugf(true, "hasErrorKeyword=%t", hasErrorKeyword)
	}
	if hasSuccessKeyword, ok := diag["hasSuccessKeyword"].(bool); ok {
		firewallDebugf(true, "hasSuccessKeyword=%t", hasSuccessKeyword)
	}
	if tail, ok := diag["existingUniqueTail"].([]interface{}); ok && len(tail) > 0 {
		firewallDebugf(true, "existingUniqueTail=%v", tail)
	}
	if required, ok := diag["requiredSnapshot"].(map[string]interface{}); ok && len(required) > 0 {
		firewallDebugf(true, "requiredSnapshot=%v", required)
	}
}

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
	}`, []interface{}{firewallTabAPIBase, tab, idx})
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
	}`, []interface{}{firewallTabAPIBase, idx})
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
		const targets = [];
		const seen = new Set();
		const pushTarget = (v) => {
			const n = normalize(v).toUpperCase();
			if (!n || seen.has(n)) return;
			seen.add(n);
			targets.push(n);
		};

		for (const el of doc.querySelectorAll("[name='bot[target][]']")) {
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
	}`, []interface{}{firewallTabAPIBase, idx})
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
	if len(targets) == 0 && len(rows) > 0 {
		targets = inferFirewallBotTargetsFromRows(rows)
	}
	return targets, rows, nil
}

func normalizeFirewallInternationalTargetValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		token := strings.ToUpper(strings.TrimSpace(value))
		if token == "" {
			continue
		}
		canonical, ok := firewallInternationalAliasMap[token]
		if !ok {
			continue
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		result = append(result, canonical)
	}
	return result
}

func normalizeFirewallBotTargetValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		token := strings.ToUpper(strings.TrimSpace(value))
		if token == "" {
			continue
		}
		canonical, ok := firewallBotAliasMap[token]
		if !ok {
			continue
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		result = append(result, canonical)
	}
	return result
}

func equalInternationalTargets(expected, actual []string) bool {
	expectedSet := toUpperSet(normalizeFirewallInternationalTargetValues(expected))
	actualSet := toUpperSet(normalizeFirewallInternationalTargetValues(actual))

	if expectedSet["FOREIGN"] && len(expectedSet) > 1 {
		return false
	}
	if actualSet["FOREIGN"] && len(actualSet) > 1 {
		return false
	}

	return equalStringSet(expectedSet, actualSet)
}

func equalFirewallBotTargets(expected, actual []string) bool {
	expectedSet := toUpperSet(normalizeFirewallBotTargetValues(expected))
	actualSet := toUpperSet(normalizeFirewallBotTargetValues(actual))

	if expectedSet["ALL"] && len(expectedSet) > 1 {
		return false
	}
	if actualSet["ALL"] && len(actualSet) > 1 {
		return false
	}

	return equalStringSet(expectedSet, actualSet)
}

func estimateFirewallRuleDataRows(rows [][]string) int {
	if len(rows) == 0 {
		return 0
	}
	count := 0
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		if i == 0 {
			head := strings.ToLower(strings.Join(row, " "))
			if strings.Contains(head, "프로토콜") || strings.Contains(head, "port") || strings.Contains(head, "서비스") {
				continue
			}
		}
		count++
	}
	return count
}

func inferInternationalTargetsFromRows(rows [][]string) []string {
	joined := strings.ToUpper(strings.Join(flattenRows(rows), " "))
	if joined == "" {
		return nil
	}

	result := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, code := range firewallInternationalAllowed {
		hints, ok := firewallInternationalDisplayHints[code]
		if !ok || len(hints) == 0 {
			hints = []string{code}
		}
		for _, hint := range hints {
			h := strings.ToUpper(strings.TrimSpace(hint))
			if h == "" {
				continue
			}
			if strings.Contains(joined, h) {
				if !seen[code] {
					seen[code] = true
					result = append(result, code)
				}
				break
			}
		}
	}
	return result
}

func inferFirewallBotTargetsFromRows(rows [][]string) []string {
	joined := strings.ToUpper(strings.Join(flattenRows(rows), " "))
	if joined == "" {
		return nil
	}

	result := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, code := range firewallBotAllowed {
		hints, ok := firewallBotDisplayHints[code]
		if !ok || len(hints) == 0 {
			hints = []string{code}
		}
		for _, hint := range hints {
			h := strings.ToUpper(strings.TrimSpace(hint))
			if h == "" {
				continue
			}
			if strings.Contains(joined, h) {
				if !seen[code] {
					seen[code] = true
					result = append(result, code)
				}
				break
			}
		}
	}
	return result
}

func normalizeRuleIPInput(raw string) string {
	s := strings.Join(strings.Fields(strings.TrimSpace(raw)), "")
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, ",") || strings.HasSuffix(s, ",") {
		s = strings.Trim(s, ",")
	}
	return s
}

func canonicalRuleIPForCompare(raw string) string {
	s := normalizeRuleIPInput(raw)
	if s == "" {
		return ""
	}

	if s == "0.0.0.0" || s == "0.0.0.0/0" {
		return "0.0.0.0/0"
	}

	if strings.Contains(s, "/") {
		parts := strings.SplitN(s, "/", 2)
		if len(parts) != 2 {
			return s
		}
		ipPart := strings.TrimSpace(parts[0])
		maskPart := strings.TrimSpace(parts[1])
		if ip := net.ParseIP(ipPart); ip != nil {
			if v4 := ip.To4(); v4 != nil {
				ipPart = v4.String()
			} else {
				ipPart = ip.String()
			}
		}
		return ipPart + "/" + maskPart
	}

	if ip := net.ParseIP(s); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4.String() + "/32"
		}
		return ip.String()
	}

	return s
}

func ruleIPEquivalent(a, b string) bool {
	ca := canonicalRuleIPForCompare(a)
	cb := canonicalRuleIPForCompare(b)
	if ca == "" || cb == "" {
		return false
	}
	if ca == cb {
		return true
	}
	return strings.Contains(strings.ToUpper(normalizeRuleIPInput(a)), strings.ToUpper(normalizeRuleIPInput(b))) ||
		strings.Contains(strings.ToUpper(normalizeRuleIPInput(b)), strings.ToUpper(normalizeRuleIPInput(a)))
}

func hasRuleRow(rows [][]string, protocol, port, ruleIP string) bool {
	protocol = strings.ToUpper(strings.TrimSpace(protocol))
	port = strings.TrimSpace(port)
	ruleIP = normalizeRuleIPInput(ruleIP)

	for i, row := range rows {
		if len(row) == 0 {
			continue
		}

		// 첫 행이 헤더인 경우가 많아 제외
		if i == 0 && strings.Contains(strings.Join(row, " "), "프로토콜") {
			continue
		}

		if len(row) >= 4 {
			rowProtocol := strings.ToUpper(strings.TrimSpace(row[1]))
			rowPort := strings.TrimSpace(row[2])
			rowIP := strings.TrimSpace(row[3])
			if rowProtocol == protocol && rowPort == port && ruleIPEquivalent(rowIP, ruleIP) {
				return true
			}
			continue
		}

		joined := strings.ToUpper(strings.Join(row, " "))
		if strings.Contains(joined, protocol) && strings.Contains(joined, port) {
			for _, candidate := range []string{
				normalizeRuleIPInput(ruleIP),
				canonicalRuleIPForCompare(ruleIP),
				strings.TrimSuffix(canonicalRuleIPForCompare(ruleIP), "/32"),
			} {
				candidate = strings.ToUpper(strings.TrimSpace(candidate))
				if candidate != "" && strings.Contains(joined, candidate) {
					return true
				}
			}
		}
	}

	return false
}

func stringifyJSMatrix(raw interface{}) [][]string {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rowValues, ok := item.([]interface{})
		if !ok {
			if text, ok := item.(string); ok {
				text = strings.TrimSpace(text)
				if text != "" {
					rows = append(rows, []string{text})
				}
			}
			continue
		}

		row := make([]string, 0, len(rowValues))
		for _, value := range rowValues {
			text, ok := value.(string)
			if !ok {
				continue
			}
			row = append(row, strings.TrimSpace(text))
		}

		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	return rows
}

func printFirewallTabRows(tabLabel, name, idx string, rows [][]string) {
	fmt.Printf("\n=== [ELCAP %s 정책 | %s | IDX: %s] ===\n", tabLabel, strings.TrimSpace(name), idx)

	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	if maxCols <= 1 {
		for i, row := range rows {
			if len(row) == 0 {
				continue
			}
			fmt.Printf("[%d] %s\n", i+1, strings.TrimSpace(row[0]))
		}
		fmt.Println("========================================")
		return
	}

	normalized := normalizeRows(rows, maxCols)
	header := normalized[0]
	data := normalized[1:]

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintln(writer, strings.Join(header, "\t"))
	fmt.Fprintln(writer, strings.Join(makeDivider(header), "\t"))
	for _, row := range data {
		fmt.Fprintln(writer, strings.Join(row, "\t"))
	}
	_ = writer.Flush()

	fmt.Printf("총 %d건\n", len(data))
	fmt.Println("========================================")
}

func normalizeRows(rows [][]string, maxCols int) [][]string {
	result := make([][]string, 0, len(rows))
	for _, row := range rows {
		normalized := make([]string, maxCols)
		copy(normalized, row)
		result = append(result, normalized)
	}
	return result
}

func makeDivider(header []string) []string {
	divider := make([]string, len(header))
	for i := range header {
		divider[i] = "----"
	}
	return divider
}
