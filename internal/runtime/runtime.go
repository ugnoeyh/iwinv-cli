package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"time"

	"iwinv-cli/internal/auth"
	"iwinv-cli/internal/envfile"
	"iwinv-cli/internal/ui"

	"github.com/playwright-community/playwright-go"
)

const (
	stateFile          = "state.json"
	loginURL           = "https://www.iwinv.kr/member/"
	instanceConsoleURL = "https://console.iwinv.kr/instance"
)

type Runtime struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	ctx     playwright.BrowserContext
}

func New(resetLogin bool, headed bool) (*Runtime, error) {
	pw, err := startPlaywright()
	if err != nil {
		return nil, err
	}

	browser, err := launchChromiumWithRecovery(pw, headed)
	if err != nil {
		_ = pw.Stop()
		return nil, fmt.Errorf("브라우저 실행 실패: %w", err)
	}

	runtime := &Runtime{
		pw:      pw,
		browser: browser,
	}

	if resetLogin {
		_ = os.Remove(stateFile)
		fmt.Println("🔄 기존 로그인 세션을 초기화했습니다.")
	}

	ctx, err := createBrowserContext(browser)
	if err != nil {
		runtime.Close()
		return nil, err
	}

	runtime.ctx = ctx
	return runtime, nil
}

func startPlaywright() (*playwright.Playwright, error) {
	if err := ensurePlaywrightInstalled(); err != nil {
		return nil, err
	}

	pw, err := playwright.Run()
	if err == nil {
		return pw, nil
	}

	fmt.Printf("⚠️ Playwright 실행 준비가 불완전하여 재설치를 시도합니다: %v\n", err)
	if installErr := installPlaywrightChromium(); installErr != nil {
		return nil, fmt.Errorf("Playwright 실행 실패: %v (설치 실패: %w)", err, installErr)
	}

	pw, err = playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("Playwright 실행 실패: %w", err)
	}

	return pw, nil
}

func launchChromiumWithRecovery(pw *playwright.Playwright, headed bool) (playwright.Browser, error) {
	launch := func() (playwright.Browser, error) {
		return pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(!headed),
		})
	}

	browser, err := launch()
	if err == nil {
		return browser, nil
	}

	fmt.Printf("⚠️ Chromium 실행 실패로 Playwright 재설치를 시도합니다: %v\n", err)
	if installErr := installPlaywrightChromium(); installErr != nil {
		return nil, fmt.Errorf("%v (재설치 실패: %w)", err, installErr)
	}

	browser, retryErr := launch()
	if retryErr != nil {
		return nil, retryErr
	}
	return browser, nil
}

func ensurePlaywrightInstalled() error {
	stampPath := playwrightInstallStampPath()
	if stampPath != "" {
		if _, err := os.Stat(stampPath); err == nil {
			return nil
		}
	}

	fmt.Println("Playwright(Chromium) 초기 설치를 진행합니다...")
	if err := installPlaywrightChromium(); err != nil {
		return err
	}

	if stampPath != "" {
		if err := os.MkdirAll(filepath.Dir(stampPath), 0o755); err == nil {
			_ = os.WriteFile(stampPath, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644)
		}
	}

	fmt.Println("Playwright 설치 확인 완료")
	return nil
}

func installPlaywrightChromium() error {
	installOpt := &playwright.RunOptions{
		Browsers: []string{"chromium"},
		Verbose:  false,
	}
	return playwright.Install(installOpt)
}

func playwrightInstallStampPath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		return ""
	}
	return filepath.Join(cacheDir, "iwinv-cli", "playwright", fmt.Sprintf("%s-%s.stamp", stdruntime.GOOS, stdruntime.GOARCH))
}

func (rt *Runtime) NewPage() (playwright.Page, error) {
	return rt.ctx.NewPage()
}

func (rt *Runtime) SaveState() error {
	_, err := rt.ctx.StorageState(stateFile)
	return err
}

func (rt *Runtime) Close() {
	if rt.browser != nil {
		_ = rt.browser.Close()
	}
	if rt.pw != nil {
		_ = rt.pw.Stop()
	}
}

func createBrowserContext(browser playwright.Browser) (playwright.BrowserContext, error) {
	if _, err := os.Stat(stateFile); err == nil {
		fmt.Println("💾 저장된 세션(state.json)을 사용합니다.")
		ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
			StorageStatePath: playwright.String(stateFile),
		})
		if err != nil {
			return nil, fmt.Errorf("저장된 세션 로드 실패: %w", err)
		}

		valid, err := isSessionValid(ctx)
		if err != nil {
			_ = ctx.Close()
			return nil, err
		}
		if valid {
			return ctx, nil
		}

		_ = ctx.Close()
		_ = os.Remove(stateFile)
		fmt.Println("⚠️ 저장된 세션이 만료되었습니다. 새로 로그인합니다.")
	}

	return loginAndPersist(browser)
}

func loginAndPersist(browser playwright.Browser) (playwright.BrowserContext, error) {
	fmt.Println("🚀 로그인이 필요합니다.")

	userID, userPW := resolveLoginCredentials()
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(userPW) == "" {
		return nil, fmt.Errorf("로그인 정보가 비어 있습니다. 입력값 또는 .env의 %s/%s 값을 확인하세요", envfile.LoginIDKey, envfile.PasswordKey)
	}

	ctx, err := browser.NewContext()
	if err != nil {
		return nil, fmt.Errorf("브라우저 컨텍스트 생성 실패: %w", err)
	}
	created := true
	defer func() {
		if created {
			_ = ctx.Close()
		}
	}()

	page, err := ctx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("로그인 페이지 생성 실패: %w", err)
	}
	defer page.Close()

	dialogMessages := make(chan string, 4)
	page.On("dialog", func(d playwright.Dialog) {
		msg := strings.TrimSpace(d.Message())
		if msg != "" {
			select {
			case dialogMessages <- msg:
			default:
			}
		}
		_ = d.Accept()
	})

	if _, err := page.Goto(loginURL); err != nil {
		return nil, fmt.Errorf("로그인 페이지 접속 실패: %w", err)
	}
	if err := page.Locator("input[name='id']").Fill(userID); err != nil {
		return nil, fmt.Errorf("ID 입력 실패: %w", err)
	}
	if err := page.Locator("input[name='pw']").Fill(userPW); err != nil {
		return nil, fmt.Errorf("비밀번호 입력 실패: %w", err)
	}
	if err := page.Locator("input[name='pw']").Press("Enter"); err != nil {
		return nil, fmt.Errorf("로그인 제출 실패: %w", err)
	}

	if err := waitForLoginCompletion(page, 12*time.Second, dialogMessages); err != nil {
		return nil, err
	}

	if _, err := ctx.StorageState(stateFile); err != nil {
		return nil, fmt.Errorf("세션 저장 실패: %w", err)
	}

	fmt.Println("✅ 로그인 성공 및 세션 저장 완료")
	created = false
	return ctx, nil
}

func resolveLoginCredentials() (string, string) {
	userID := strings.TrimSpace(os.Getenv(envfile.LoginIDKey))
	userPW := strings.TrimSpace(os.Getenv(envfile.PasswordKey))
	if userPW == "" {
		legacyPW := strings.TrimSpace(os.Getenv(envfile.LegacyLoginPasswordKey))
		if legacyPW != "" {
			fmt.Printf("⚠️ %s 는 더 이상 권장되지 않습니다. %s 를 사용하세요.\n", envfile.LegacyLoginPasswordKey, envfile.PasswordKey)
			userPW = legacyPW
		}
	}

	if isPlaceholderCredential(userID) {
		fmt.Printf("⚠️ %s 값이 예시값입니다. 실제 ID로 바꿔주세요.\n", envfile.LoginIDKey)
		userID = ""
	}
	if isPlaceholderCredential(userPW) {
		fmt.Printf("⚠️ %s 값이 예시값입니다. 실제 비밀번호로 바꿔주세요.\n", envfile.PasswordKey)
		userPW = ""
	}

	if userID != "" && userPW != "" {
		fmt.Printf("🔐 .env 로그인 정보(%s/%s)를 사용합니다.\n", envfile.LoginIDKey, envfile.PasswordKey)
		return userID, userPW
	}

	if userID == "" {
		userID = ui.PromptLine("👤 ID: ")
	}
	if userPW == "" {
		userPW = ui.PromptLine("🔑 PW: ")
	}

	return strings.TrimSpace(userID), strings.TrimSpace(userPW)
}

func isPlaceholderCredential(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "", "your-id", "your-password", "id", "password", "changeme", "replace-me":
		return v != ""
	default:
		return false
	}
}

func isSessionValid(ctx playwright.BrowserContext) (bool, error) {
	page, err := ctx.NewPage()
	if err != nil {
		return false, fmt.Errorf("세션 검사 페이지 생성 실패: %w", err)
	}
	defer page.Close()

	if _, err := page.Goto(instanceConsoleURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(12000),
	}); err != nil {
		return false, fmt.Errorf("세션 검사 접속 실패: %w", err)
	}

	page.WaitForTimeout(500)

	onLogin, err := isLoginPage(page)
	if err != nil {
		return false, fmt.Errorf("로그인 페이지 판별 실패: %w", err)
	}
	return !onLogin, nil
}

func waitForLoginCompletion(page playwright.Page, timeout time.Duration, dialogMessages <-chan string) error {
	deadline := time.Now().Add(timeout)
	// 서버 응답이 도착하기까지의 유예 시간 (응답 전에는 fail-fast 타이머를 시작하지 않음)
	const submissionGrace = 500 * time.Millisecond
	// 유예 기간 이후 로그인 페이지에 머물면 실패로 간주하는 시간
	const loginFailFastWindow = 1500 * time.Millisecond

	submittedAt := time.Now()
	var stayedOnLoginSince time.Time

	if msg := drainDialogFailure(dialogMessages); msg != "" {
		return fmt.Errorf("로그인 실패: %s", msg)
	}

	for {
		if msg := drainDialogFailure(dialogMessages); msg != "" {
			return fmt.Errorf("로그인 실패: %s", msg)
		}

		onLogin, err := isLoginPage(page)
		if err != nil {
			return fmt.Errorf("로그인 결과 확인 실패: %w", err)
		}
		if !onLogin {
			return nil
		}

		// 유예 기간이 지난 뒤에만 fail-fast 타이머를 시작
		if time.Since(submittedAt) >= submissionGrace {
			if stayedOnLoginSince.IsZero() {
				stayedOnLoginSince = time.Now()
			}
			if msg, detectErr := detectLoginFailureMessage(page); detectErr == nil && msg != "" {
				return fmt.Errorf("로그인 실패: %s", msg)
			}
			if time.Since(stayedOnLoginSince) >= loginFailFastWindow {
				return fmt.Errorf("로그인 실패: 아이디/비밀번호를 확인하세요")
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("로그인 실패: 세션이 생성되지 않았습니다. ID/PW를 확인하거나 --login으로 다시 시도하세요")
		}

		page.WaitForTimeout(150)
	}
}

func drainDialogFailure(dialogMessages <-chan string) string {
	if dialogMessages == nil {
		return ""
	}

	for {
		select {
		case msg := <-dialogMessages:
			text := strings.TrimSpace(msg)
			if text == "" {
				return "로그인 중 알림창이 표시되었습니다."
			}
			return text
		default:
			return ""
		}
	}
}

func detectLoginFailureMessage(page playwright.Page) (string, error) {
	raw, err := page.Evaluate(`() => {
		function isVisible(el) {
			if (!el) return false;
			const style = window.getComputedStyle(el);
			if (style.display === "none" || style.visibility === "hidden") return false;
			const rect = el.getBoundingClientRect();
			return rect.width > 0 && rect.height > 0;
		}

		function normalize(text) {
			return (text || "").replace(/\s+/g, " ").trim();
		}

		const hints = new Set();
		const pushText = (text) => {
			const value = normalize(text);
			if (!value) return;
			if (value.length > 220) return;
			hints.add(value);
		};

		const selectors = [
			"[role='alert']",
			"[aria-live='polite']",
			"[aria-live='assertive']",
			".alert",
			".alert-danger",
			".text-danger",
			".text-red-500",
			".error",
			".error-message",
			".invalid-feedback",
			".login-error",
			".toast-message",
			".swal2-html-container"
		];

		for (const selector of selectors) {
			for (const el of Array.from(document.querySelectorAll(selector))) {
				if (!isVisible(el)) continue;
				pushText(el.textContent || el.innerText || "");
			}
		}

		const form = document.querySelector("form");
		if (form) {
			for (const el of Array.from(form.querySelectorAll("div, p, span, li"))) {
				if (!isVisible(el)) continue;
				pushText(el.textContent || el.innerText || "");
			}
		}

		return Array.from(hints);
	}`)
	if err != nil {
		return "", err
	}

	items, ok := raw.([]interface{})
	if !ok {
		return "", nil
	}

	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if isLikelyLoginFailureText(text) {
			return text, nil
		}
	}

	return "", nil
}

func isLikelyLoginFailureText(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	compact := strings.ReplaceAll(normalized, " ", "")

	strongFailureTokens := []string{
		"일치하지", "일치하지않", "틀렸", "잘못", "invalid", "incorrect", "wrong", "mismatch", "notmatch",
		"아이디혹은비밀번호", "아이디또는비밀번호", "회원정보를확인", "로그인정보를확인", "존재하지", "없는계정",
	}
	for _, token := range strongFailureTokens {
		if strings.Contains(compact, token) {
			return true
		}
	}

	weakFailureTokens := []string{
		"오류", "실패", "다시입력", "재입력", "확인", "error", "failed", "retry",
	}
	credentialTokens := []string{
		"아이디", "비밀번호", "id", "password", "username", "passwd", "pw", "로그인", "회원정보", "계정",
	}

	hasWeakFailure := false
	for _, token := range weakFailureTokens {
		if strings.Contains(compact, token) {
			hasWeakFailure = true
			break
		}
	}
	if !hasWeakFailure {
		return false
	}

	for _, token := range credentialTokens {
		if strings.Contains(compact, token) {
			return true
		}
	}

	return false
}

func isLoginPage(page playwright.Page) (bool, error) {
	return auth.IsLoginPage(page)
}
