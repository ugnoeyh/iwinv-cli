package runtime

import (
	"fmt"
	"os"
	"time"

	"iwinv-cli/internal/auth"
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

func New(resetLogin bool) (*Runtime, error) {
	pw, err := startPlaywright()
	if err != nil {
		return nil, err
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
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
	pw, err := playwright.Run()
	if err == nil {
		return pw, nil
	}

	if installErr := playwright.Install(); installErr != nil {
		return nil, fmt.Errorf("Playwright 실행 실패: %v (설치 실패: %w)", err, installErr)
	}

	pw, err = playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("Playwright 실행 실패: %w", err)
	}

	return pw, nil
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

	userID := ui.PromptLine("👤 ID: ")
	userPW := ui.PromptLine("🔑 PW: ")

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

	page.On("dialog", func(d playwright.Dialog) { go d.Accept() })

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

	if err := waitForLoginCompletion(page, 30*time.Second); err != nil {
		return nil, err
	}

	if _, err := ctx.StorageState(stateFile); err != nil {
		return nil, fmt.Errorf("세션 저장 실패: %w", err)
	}

	fmt.Println("✅ 로그인 성공 및 세션 저장 완료")
	created = false
	return ctx, nil
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

func waitForLoginCompletion(page playwright.Page, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		onLogin, err := isLoginPage(page)
		if err != nil {
			return fmt.Errorf("로그인 결과 확인 실패: %w", err)
		}
		if !onLogin {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("로그인 실패: 세션이 생성되지 않았습니다. ID/PW를 확인하거나 --login으로 다시 시도하세요")
		}

		page.WaitForTimeout(200)
	}
}

func isLoginPage(page playwright.Page) (bool, error) {
	return auth.IsLoginPage(page)
}
