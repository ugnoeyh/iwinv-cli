package console

import (
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

func RunOpenSupportRequestWrite(page playwright.Page) error {
	fmt.Println("🚀 기술지원 작성 페이지로 이동 중입니다... (직접 진입)")

	if _, err := page.Goto(supportRequestWrite, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("기술지원 작성 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "기술지원 작성 페이지 접속"); err != nil {
		return err
	}

	path, pathErr := getCurrentPathname(page)
	if pathErr != nil {
		return pathErr
	}

	if strings.HasPrefix(path, "/support/request/agree") {
		_, diag, err := submitSupportRequestAgreement(page)
		if err != nil {
			currentPath, _ := diag["currentPath"].(string)
			if currentPath != "" {
				return fmt.Errorf("기술지원 동의 제출 실패: %w (path=%s)", err, currentPath)
			}
			return fmt.Errorf("기술지원 동의 제출 실패: %w", err)
		}
	}
	if err := waitForSupportRequestWriteReady(page, 12*time.Second); err != nil {
		if _, navErr := page.Goto(supportRequestWrite, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); navErr == nil {
			_ = ensureAuthenticated(page, "기술지원 작성 페이지 접속")
			if retryErr := waitForSupportRequestWriteReady(page, 8*time.Second); retryErr == nil {
				fmt.Println("✅ 기술지원 작성 페이지로 이동 완료")
				return nil
			}
		}
		currentPath, _ := getCurrentPathname(page)
		if currentPath != "" {
			return fmt.Errorf("기술지원 작성 페이지 진입 확인 실패: path=%s", currentPath)
		}
		return err
	}

	fmt.Println("✅ 기술지원 작성 페이지로 이동 완료")
	return nil
}

func getCurrentPathname(page playwright.Page) (string, error) {
	raw, err := page.Evaluate(`() => String(location.pathname || "").trim()`)
	if err != nil {
		return "", fmt.Errorf("현재 페이지 경로 확인 실패: %w", err)
	}
	path, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("현재 페이지 경로 형식을 해석할 수 없습니다")
	}
	return strings.TrimSpace(path), nil
}

func submitSupportRequestAgreement(page playwright.Page) (bool, map[string]interface{}, error) {
	diag := map[string]interface{}{}

	formDiag, formErr := submitSupportRequestAgreementByForm(page)
	if formDiag != nil {
		diag["formAttempt"] = formDiag
	}
	if formErr != nil {
		diag["formError"] = formErr.Error()
	}

	path, pathErr := getCurrentPathname(page)
	if pathErr == nil {
		diag["pathAfterForm"] = path
		if isSupportRequestWritePath(path) {
			diag["currentPath"] = path
			return true, diag, nil
		}
	} else {
		diag["pathAfterFormError"] = pathErr.Error()
	}

	// 폼 제출만으로 반영되지 않는 케이스가 있어 agree.json 선조회 + POST 흐름을 fallback으로 수행한다.
	requestDiag, requestErr := submitSupportRequestAgreementByRequestFlow(page)
	if requestDiag != nil {
		diag["requestFlowAttempt"] = requestDiag
	}
	if requestErr != nil {
		diag["requestFlowError"] = requestErr.Error()
		_ = page.WaitForURL("**/support/request/**", playwright.PageWaitForURLOptions{
			Timeout: playwright.Float(2500),
		})
		currentPath, pathErr := getCurrentPathname(page)
		if pathErr == nil {
			diag["currentPath"] = currentPath
			if isSupportRequestWritePath(currentPath) && isExecutionContextDestroyedError(requestErr) {
				return true, diag, nil
			}
		}
		return false, diag, fmt.Errorf("동의 요청 흐름 실패: %w", requestErr)
	}

	_ = page.WaitForURL("**/support/request/**", playwright.PageWaitForURLOptions{
		Timeout: playwright.Float(4000),
	})

	finalPath, finalPathErr := getCurrentPathname(page)
	if finalPathErr == nil {
		diag["currentPath"] = finalPath
	}
	if finalPathErr == nil && isSupportRequestWritePath(finalPath) {
		return true, diag, nil
	}
	return true, diag, nil
}

func submitSupportRequestAgreementByForm(page playwright.Page) (map[string]interface{}, error) {
	raw, err := page.Evaluate(`() => {
		const agree1 = document.querySelector("input[name='agree1']");
		const agree2 = document.querySelector("input[name='agree2']");
		const form = (agree1 && agree1.form) || (agree2 && agree2.form) || document.querySelector("form[action*='/support/request/write']");
		const diag = {
			currentURL: location.href,
			hasAgree1: !!agree1,
			hasAgree2: !!agree2,
			hasForm: !!form
		};

		if (!agree1 || !agree2 || !form) {
			return { success: false, reason: "동의 폼 요소를 찾지 못했습니다.", diag };
		}

		if (agree1.type === "checkbox") agree1.checked = true;
		if (agree2.type === "checkbox") agree2.checked = true;
		agree1.dispatchEvent(new Event("input", { bubbles: true }));
		agree1.dispatchEvent(new Event("change", { bubbles: true }));
		agree2.dispatchEvent(new Event("input", { bubbles: true }));
		agree2.dispatchEvent(new Event("change", { bubbles: true }));

		diag.formAction = String(form.getAttribute("action") || "");
		diag.formMethod = String(form.getAttribute("method") || "GET").toUpperCase();

		const submitter = form.querySelector("button[type='submit'],input[type='submit']");
		diag.hasSubmitter = !!submitter;

		try {
			if (submitter && typeof submitter.click === "function") {
				submitter.click();
			} else if (typeof form.requestSubmit === "function") {
				form.requestSubmit();
			} else {
				form.submit();
			}
			return { success: true, diag };
		} catch (e) {
			return { success: false, reason: String((e && e.message) || e), diag };
		}
	}`)
	if err != nil {
		errLower := strings.ToLower(err.Error())
		if strings.Contains(errLower, "execution context was destroyed") {
			diag := map[string]interface{}{"navigated": true}
			return diag, nil
		}
		return nil, fmt.Errorf("동의 폼 제출 스크립트 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("동의 폼 제출 응답 형식을 해석할 수 없습니다")
	}

	success, _ := res["success"].(bool)
	diag, _ := res["diag"].(map[string]interface{})
	if !success {
		reason, _ := res["reason"].(string)
		if reason != "" {
			return diag, fmt.Errorf("%s", reason)
		}
		return diag, fmt.Errorf("동의 폼 제출 실패")
	}

	_ = page.WaitForURL("**/support/request/**", playwright.PageWaitForURLOptions{
		Timeout: playwright.Float(3500),
	})
	return diag, nil
}

func submitSupportRequestAgreementByRequestFlow(page playwright.Page) (map[string]interface{}, error) {
	raw, err := page.Evaluate(`async ([agreeJSONURL, writeURL]) => {
		const token =
			String((document.querySelector("input[name='_token']") && document.querySelector("input[name='_token']").value) || "").trim() ||
			String((document.querySelector('meta[name="csrf-token"]') && document.querySelector('meta[name="csrf-token"]').content) || "").trim();
		const typeRaw = String((document.querySelector("input[name='_type']") && document.querySelector("input[name='_type']").value) || "").trim();
		const typeValue = typeRaw || JSON.stringify({ agree1: "checkbox", agree2: "checkbox" });

		const diag = {
			currentURL: location.href,
			agreeJSONURL,
			writeURL,
			tokenLength: token.length
		};
		if (!token) {
			return { success: false, reason: "_token이 비어 있습니다.", diag };
		}

		try {
			const agreeRes = await fetch(agreeJSONURL, {
				method: "GET",
				headers: {
					"accept": "application/json, text/javascript, */*; q=0.01",
					"x-csrf-token": token,
					"x-requested-with": "XMLHttpRequest"
				},
				credentials: "same-origin"
			});
			diag.agreeJSONStatus = agreeRes.status;
		} catch (e) {
			diag.agreeJSONError = String((e && e.message) || e);
		}

		const params = new URLSearchParams();
		params.set("agree1", "1");
		params.set("agree2", "1");
		params.set("_token", token);
		params.set("_type", typeValue);

		const postRes = await fetch(writeURL, {
			method: "POST",
			headers: {
				"accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
				"content-type": "application/x-www-form-urlencoded; charset=UTF-8"
			},
			body: params.toString(),
			credentials: "same-origin",
			redirect: "follow"
		});

		const body = await postRes.text();
		const bodyLower = body.toLowerCase();
		const looksLikeLogin = bodyLower.includes("name=\"id\"") && bodyLower.includes("name=\"pw\"");
		const looksLikeAgree = bodyLower.includes("agree1") && bodyLower.includes("agree2") && bodyLower.includes("/support/request/write");
		const success = postRes.status >= 200 && postRes.status < 400 && !looksLikeLogin && !looksLikeAgree;

		diag.postStatus = postRes.status;
		diag.postURL = postRes.url || "";
		diag.postLooksLikeLogin = looksLikeLogin;
		diag.postLooksLikeAgree = looksLikeAgree;
		diag.postBodyPreview = body.slice(0, 320);

		if (!success) {
			return { success: false, reason: "동의 POST 응답이 write 페이지로 확정되지 않았습니다.", diag };
		}
		return { success: true, diag };
	}`, []interface{}{supportRequestAgreeJSON, supportRequestWrite})
	if err != nil {
		return nil, fmt.Errorf("동의 요청 흐름 실행 실패: %w", err)
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("동의 요청 흐름 응답 형식을 해석할 수 없습니다")
	}

	success, _ := res["success"].(bool)
	diag, _ := res["diag"].(map[string]interface{})
	if !success {
		reason, _ := res["reason"].(string)
		if reason == "" {
			reason = "동의 요청 흐름 실패"
		}
		return diag, fmt.Errorf("%s", reason)
	}

	return diag, nil
}

func isSupportRequestWritePath(path string) bool {
	path = strings.TrimSpace(path)
	return path == "/support/request/write" || strings.HasPrefix(path, "/support/request/write/")
}

func isExecutionContextDestroyedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context was destroyed") ||
		strings.Contains(msg, "execution context") && strings.Contains(msg, "destroyed")
}

func waitForSupportRequestWriteReady(page playwright.Page, timeout time.Duration) error {
	var lastState string
	err := waitForCondition(page, timeout, func() (bool, error) {
		raw, err := page.Evaluate(`() => {
			const path = String(location.pathname || "");
			return { path };
		}`)
		if err != nil {
			return false, err
		}

		obj, ok := raw.(map[string]interface{})
		if !ok {
			return false, fmt.Errorf("기술지원 작성 페이지 상태를 해석할 수 없습니다")
		}

		path := strings.TrimSpace(toString(obj["path"]))
		lastState = fmt.Sprintf("path=%s", path)

		return isSupportRequestWritePath(path), nil
	})
	if err != nil {
		if lastState != "" {
			return fmt.Errorf("기술지원 작성 페이지 진입 확인 실패: %s", lastState)
		}
		return fmt.Errorf("기술지원 작성 페이지 진입 확인 실패: %w", err)
	}

	return nil
}
