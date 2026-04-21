package console

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"net/url"
	"regexp"
	"strings"
	"time"
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

type firewallPolicyReference struct {
	Raw           string
	Idx           string
	ResolvedName  string
	TitleFallback string
}

type firewallTargetsFetcher func(page playwright.Page, idx string) ([]string, [][]string, error)

type firewallTargetsUpdater func(page playwright.Page, policyRef string, targets []string, debug bool, actionLabel string) error

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

var firewallBotAllExpanded = []string{
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

func resolveFirewallPolicyReference(page playwright.Page, policyRef string, debug bool, resolveNameByIdx bool) (firewallPolicyReference, error) {
	ref := firewallPolicyReference{
		Raw:          strings.TrimSpace(policyRef),
		Idx:          strings.TrimSpace(policyRef),
		ResolvedName: strings.TrimSpace(policyRef),
	}
	if !numericIDRegex.MatchString(ref.Raw) {
		if _, err := page.Goto(firewallURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return firewallPolicyReference{}, fmt.Errorf("ELCAP 방화벽 페이지 접속 실패: %w", err)
		}
		if err := ensureAuthenticated(page, "ELCAP 방화벽 페이지 접속"); err != nil {
			return firewallPolicyReference{}, err
		}

		resolvedIdx, resolvedPolicyName, err := resolveFirewallPolicyIdx(page, ref.Raw)
		if err != nil {
			return firewallPolicyReference{}, err
		}
		ref.Idx = resolvedIdx
		ref.ResolvedName = resolvedPolicyName
		firewallDebugf(debug, "resolved policy | name=%s idx=%s", ref.ResolvedName, ref.Idx)
	}

	if resolveNameByIdx && numericIDRegex.MatchString(strings.TrimSpace(ref.ResolvedName)) {
		if nameByIdx, nameErr := resolveFirewallPolicyNameByIdx(page, ref.Idx); nameErr == nil && strings.TrimSpace(nameByIdx) != "" {
			ref.ResolvedName = strings.TrimSpace(nameByIdx)
			firewallDebugf(debug, "resolved policy name by idx | idx=%s name=%s", ref.Idx, ref.ResolvedName)
		} else if nameErr != nil {
			firewallDebugf(debug, "resolve policy name by idx warning | idx=%s err=%v", ref.Idx, nameErr)
		}
	}

	ref.TitleFallback = strings.TrimSpace(ref.ResolvedName)
	if strings.TrimSpace(ref.TitleFallback) == strings.TrimSpace(ref.Idx) {
		ref.TitleFallback = ""
	}
	return ref, nil
}

func openFirewallPolicyTabPage(page playwright.Page, idx string) error {
	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabURL, url.QueryEscape(idx))
	if _, err := page.Goto(tabPageURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("방화벽 정책 상세 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "방화벽 정책 상세 페이지 접속"); err != nil {
		return err
	}
	return nil
}

func openFirewallPolicyEditPage(page playwright.Page, idx, tab string) (string, error) {
	editPageURL := fmt.Sprintf("%s/%s/edit?tab=%s", firewallURL, url.PathEscape(idx), url.QueryEscape(tab))
	if _, err := page.Goto(editPageURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return "", fmt.Errorf("방화벽 정책 편집 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "방화벽 정책 편집 페이지 접속"); err != nil {
		return "", err
	}
	if err := waitForFirewallWriteFormReady(page, 8*time.Second); err != nil {
		return "", err
	}
	return editPageURL, nil
}

func formatFirewallTargetText(targets []string) string {
	if len(targets) == 0 {
		return "(empty)"
	}
	return strings.Join(targets, ",")
}

func runRemoveFirewallTargets(
	page playwright.Page,
	policyRef string,
	removeTargets []string,
	debug bool,
	policyLabel string,
	targetLabel string,
	debugPrefix string,
	fetcher firewallTargetsFetcher,
	updater firewallTargetsUpdater,
) error {
	policy, err := resolveFirewallPolicyReference(page, policyRef, debug, false)
	if err != nil {
		return err
	}
	if err := openFirewallPolicyTabPage(page, policy.Idx); err != nil {
		return err
	}

	currentTargets, _, err := fetcher(page, policy.Idx)
	if err != nil {
		return err
	}
	if len(currentTargets) == 0 {
		fmt.Printf("ℹ️ [%s | IDX:%s] %s 정책이 비어 있어 개별 제거를 건너뜁니다.\n", strings.TrimSpace(policy.ResolvedName), policy.Idx, policyLabel)
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
		fmt.Printf("ℹ️ [%s | IDX:%s] 요청한 %s 대상이 현재 정책에 없어 개별 제거를 건너뜁니다: %s\n", strings.TrimSpace(policy.ResolvedName), policy.Idx, targetLabel, strings.Join(removeTargets, ","))
		return nil
	}

	firewallDebugf(debug, "%s remove | idx=%s current=%v remove=%v remain=%v", debugPrefix, policy.Idx, currentTargets, removeTargets, remaining)
	return updater(page, policy.Idx, remaining, debug, "개별 제거")
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
	targets, err := parseFirewallBotTargetList(input, "--firewall-bot", true)
	if err != nil {
		return nil, err
	}
	return normalizeFirewallBotTargetValues(targets), nil
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
		fmt.Printf("⚠️ 지원하지 않는 %s 값은 건너뜁니다: %s (가능값: %s)\n",
			optionName,
			strings.Join(invalid, ", "),
			strings.Join(firewallInternationalAllowed, ","),
		)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s 에서 유효한 값이 없습니다 (가능값: %s)", optionName, strings.Join(firewallInternationalAllowed, ","))
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
		fmt.Printf("⚠️ 지원하지 않는 %s 값은 건너뜁니다: %s (가능값: %s)\n",
			optionName,
			strings.Join(invalid, ", "),
			strings.Join(firewallBotAllowed, ","),
		)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s 에서 유효한 값이 없습니다 (가능값: %s)", optionName, strings.Join(firewallBotAllowed, ","))
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
