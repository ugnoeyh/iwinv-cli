package console

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"net/url"
	"strings"
	"time"
)

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
	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabURL, url.QueryEscape(idx))
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
	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabURL, url.QueryEscape(idx))
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
	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabURL, url.QueryEscape(idx))
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
	tabPageURL := fmt.Sprintf("%s?idx=%s", firewallTabURL, url.QueryEscape(idx))
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
			if err == nil && len(expectedTargets) == 0 && hasFirewallNoPolicyHint(rows) {
				firewallDebugf(debug, "bot verify attempt=%d matched by no-policy hint", attempt)
				return true, nil
			}

			if err == nil && equalFirewallBotTargets(expectedTargets, actualTargets) {
				if len(expectedTargets) == 0 && len(actualTargets) == 0 && len(rows) == 0 {

					tabRows, tabErr := fetchFirewallTabRows(page, "bot", idx)
					if tabErr == nil {
						if hasFirewallNoPolicyHint(tabRows) {
							firewallDebugf(debug, "bot verify attempt=%d matched by tab no-policy hint", attempt)
							return true, nil
						}
						inferred := normalizeFirewallBotTargetValues(inferFirewallBotTargetsFromRows(tabRows))
						if len(inferred) == 0 {
							firewallDebugf(debug, "bot verify attempt=%d matched by tab fallback empty targets", attempt)
							return true, nil
						}
						firewallDebugf(debug, "bot verify attempt=%d tab fallback still has targets=%v", attempt, inferred)
					} else {
						lastErr = tabErr
						firewallDebugf(debug, "bot verify attempt=%d tab fallback error=%v", attempt, tabErr)
					}
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
	return hasFirewallTargetsInForm(page, expectedTargets, "bot[target][]", "검색봇 폼 검증 응답 형식을 해석할 수 없습니다")
}

func hasInternationalTargetsInForm(page playwright.Page, expectedTargets []string) (bool, []string, error) {
	return hasFirewallTargetsInForm(page, expectedTargets, "international[target][]", "국제망 폼 검증 응답 형식을 해석할 수 없습니다")
}

func hasFirewallTargetsInForm(page playwright.Page, expectedTargets []string, fieldName string, decodeErrMsg string) (bool, []string, error) {
	raw, err := page.Evaluate(`([expectedTargets, fieldName]) => {
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
		const selector = "[name='" + String(fieldName || "").trim() + "']";
		const targetInputs = Array.from(document.querySelectorAll(selector));
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
	}`, []interface{}{expectedTargets, fieldName})
	if err != nil {
		return false, nil, err
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("%s", decodeErrMsg)
	}

	targetInputCount := toInt(res["targetInputCount"])
	if targetInputCount == 0 {
		return false, nil, nil
	}

	match, _ := res["match"].(bool)
	actual := stringifyJSArray(res["actual"])
	return match, actual, nil
}
