package console

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"net/url"
	"strings"
	"time"
)

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

	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabURL, url.QueryEscape(idx))
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
