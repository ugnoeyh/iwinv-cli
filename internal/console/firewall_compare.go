package console

import (
	"net"
	"strings"
)

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
	hasAll := false
	for _, value := range values {
		token := strings.ToUpper(strings.TrimSpace(value))
		if token == "" {
			continue
		}
		canonical, ok := firewallBotAliasMap[token]
		if !ok {
			continue
		}
		if canonical == "ALL" {
			hasAll = true
			continue
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		result = append(result, canonical)
	}
	if hasAll {
		return append([]string{}, firewallBotAllExpanded...)
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

func hasFirewallNoPolicyHint(rows [][]string) bool {
	if len(rows) == 0 {
		return false
	}
	joined := strings.Join(flattenRows(rows), " ")
	return strings.Contains(joined, "설정된 정책이 없습니다") ||
		strings.Contains(joined, "정책변경 설정된 정책이 없습니다") ||
		strings.Contains(joined, "등록된 정책이 없습니다")
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
	if len(rows) == 0 || hasFirewallNoPolicyHint(rows) {
		return nil
	}

	result := make([]string, 0, 4)
	seen := map[string]bool{}
	addCode := func(code string) {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" || seen[code] {
			return
		}
		seen[code] = true
		result = append(result, code)
	}
	parseCode := func(text string) string {
		t := strings.ToUpper(strings.TrimSpace(text))
		if t == "" {
			return ""
		}
		if t == "ALL" || strings.Contains(t, "전체") {
			return "ALL"
		}
		if strings.Contains(t, "GOOGLE") || strings.Contains(t, "구글") {
			return "GOOGLE"
		}
		if strings.Contains(t, "NAVER") || strings.Contains(t, "네이버") {
			return "NAVER"
		}
		if strings.Contains(t, "DAUM") || strings.Contains(t, "다음") {
			return "DAUM"
		}
		return ""
	}

	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		joined := strings.ToUpper(strings.Join(row, " "))
		if joined == "" || strings.Contains(joined, "정책이 없습니다") {
			continue
		}
		if strings.Contains(joined, "검색 BOT") && strings.Contains(joined, "설정") {
			continue
		}
		blocked := strings.Contains(joined, "차단") || strings.Contains(joined, "BLOCK")
		if !blocked {
			continue
		}

		if code := parseCode(row[0]); code != "" {
			addCode(code)
			continue
		}
		if len(row) > 1 {
			if code := parseCode(row[1]); code != "" {
				addCode(code)
				continue
			}
		}
		if code := parseCode(joined); code != "" {
			addCode(code)
		}
	}

	return normalizeFirewallBotTargetValues(result)
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
