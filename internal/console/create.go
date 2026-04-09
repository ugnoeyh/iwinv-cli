package console

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"iwinv-cli/internal/envfile"
	"iwinv-cli/internal/output"
	"iwinv-cli/internal/ui"

	"github.com/playwright-community/playwright-go"
)

func RunCreate(page playwright.Page, region1, region2, spec, serverName string, qty int, pw, osType, osName, blockData, firewallName string) error {
	fmt.Println("\n🚀 서버 생성 프로세스 시작...")

	if err := openCreateServicePage(page); err != nil {
		return err
	}

	fmt.Println("🔎 [1단계] 리전 및 스펙 선택 중...")
	if err := selectRequiredOption(page, "리전 1차", region1XPath, region1, false); err != nil {
		return err
	}
	if region1 != "" {
		if err := waitForSelectableOptions(page, "리전 2차", region2XPath, false, 7*time.Second); err != nil {
			return err
		}
	}

	if err := selectRequiredOption(page, "리전 2차", region2XPath, region2, false); err != nil {
		return err
	}
	if region2 != "" {
		if err := waitForSelectableOptions(page, "서버 스펙", specXPath, true, 7*time.Second); err != nil {
			return err
		}
	}

	if err := selectRequiredOption(page, "서버 스펙", specXPath, spec, true); err != nil {
		return err
	}

	fmt.Println("🔎 [2단계] 운영체제 선택 중...")
	osTab := page.Locator("xpath=" + resolveOSTabXPath(osType))
	if count, _ := osTab.Count(); count > 0 {
		if err := osTab.Click(); err != nil {
			return fmt.Errorf("운영체제 탭('%s') 클릭 실패: %w", osType, err)
		}
	}
	if err := waitForSelectableOptions(page, "운영체제", osTableXPath, true, 7*time.Second); err != nil {
		return err
	}

	if err := selectRequiredOption(page, "운영체제", osTableXPath, osName, true); err != nil {
		return err
	}

	if err := applyBlockStorage(page, blockData); err != nil {
		return err
	}

	fmt.Println("🚀 옵션 페이지로 이동 대기 중...")
	if err := page.Locator("xpath=" + createNextXPath).Click(playwright.LocatorClickOptions{
		Force: playwright.Bool(true),
	}); err != nil {
		return fmt.Errorf("옵션 페이지 이동 버튼 클릭 실패: %w", err)
	}

	if err := page.WaitForURL(optionURLPattern, playwright.PageWaitForURLOptions{
		Timeout: playwright.Float(10000),
	}); err != nil {
		return fmt.Errorf("옵션 설정 단계로 넘어가지 못했습니다")
	}
	if err := waitForXPathVisible(page, "옵션 입력", serverNameXPath, 6*time.Second); err != nil {
		return err
	}

	fmt.Println("🔎 [4단계] 서버 이름 및 수량 설정 중...")
	if err := page.Locator("xpath=" + serverNameXPath).Fill(serverName); err != nil {
		return fmt.Errorf("서버 이름 입력 실패: %w", err)
	}
	if err := page.Locator("xpath=" + qtyXPath).Fill(strconv.Itoa(qty)); err != nil {
		return fmt.Errorf("서버 수량 입력 실패: %w", err)
	}

	if qty >= 2 {
		if pw == "" {
			return fmt.Errorf("수량이 2대 이상일 때는 비밀번호가 필수입니다. .env의 %s 또는 --pw 옵션을 사용하세요", envfile.PasswordKey)
		}
		if err := page.Locator("xpath=" + multiPasswordXPath).Fill(pw); err != nil {
			return fmt.Errorf("다중 생성용 비밀번호 입력 실패: %w", err)
		}
	}

	fmt.Println("🔎 [5단계] 방화벽 설정 중...")
	configureFirewall(page, firewallName)

	fmt.Println("🚀 최종 확인 페이지로 이동 대기 중...")
	if err := page.Locator("xpath=" + optionConfirmXPath).Click(playwright.LocatorClickOptions{
		Force: playwright.Bool(true),
	}); err != nil {
		return fmt.Errorf("최종 확인 페이지 이동 버튼 클릭 실패: %w", err)
	}

	if err := page.WaitForURL(confirmURLPattern, playwright.PageWaitForURLOptions{
		Timeout: playwright.Float(10000),
	}); err != nil {
		return fmt.Errorf("최종 확인 단계로 넘어가지 못했습니다")
	}
	if err := waitForTextMinLength(page, "최종 요약", summaryXPath, 10, 6*time.Second); err != nil {
		return err
	}

	fmt.Println("🔎 [6단계] 최종 정보 확인 및 생성 요청...")
	summaryText, _ := page.Locator("xpath=" + summaryXPath).InnerText()
	if strings.TrimSpace(summaryText) == "" || len(summaryText) < 10 {
		return fmt.Errorf("[치명적 오류] 최종 요약 정보를 불러오지 못했습니다")
	}

	fmt.Println("\n=== [생성 요약 정보] ===")
	fmt.Println(output.FormatCreateSummary(summaryText))
	fmt.Println("========================")
	fmt.Println()

	if !ui.ConfirmAction("❓ 위 요약 정보대로 서버를 정말 생성하시겠습니까? (y/N): ") {
		fmt.Println("🛑 사용자 취소: 서버 생성을 중단하고 프로그램을 종료합니다.")
		return nil
	}

	fmt.Println("🚀 최종 서버 생성 요청을 전송합니다...")
	if err := page.Locator("xpath=" + finalCreateXPath).Click(playwright.LocatorClickOptions{
		Force: playwright.Bool(true),
	}); err != nil {
		return fmt.Errorf("최종 생성 버튼 클릭 실패: %w", err)
	}

	if err := page.WaitForURL(successURLPattern, playwright.PageWaitForURLOptions{
		Timeout: playwright.Float(15000),
	}); err != nil {
		return fmt.Errorf("서버 생성 실패: 성공 페이지 도달 안함")
	}

	fmt.Println("✅ RESULT: Success | Action: create | 서버 생성이 정상적으로 완료되었습니다. 🎉")
	return nil
}

func applyBlockStorage(page playwright.Page, blockData string) error {
	if blockData == "" {
		return nil
	}

	fmt.Println("🔎 [3단계] 블록 스토리지 추가 중...")
	parts := strings.Split(blockData, ":")
	if len(parts) != 3 {
		return fmt.Errorf("--block 형식이 올바르지 않습니다. '타입:용량:이름' 형식을 사용하세요")
	}

	blockType := parts[0]
	blockSize := parts[1]
	blockName := parts[2]

	if err := page.Locator("xpath=" + blockTypeXPath).Click(); err != nil {
		return fmt.Errorf("블록 스토리지 타입 선택 UI 오픈 실패: %w", err)
	}

	if err := page.Locator(fmt.Sprintf("text=%s", blockType)).Last().Click(playwright.LocatorClickOptions{
		Timeout: playwright.Float(2000),
	}); err != nil {
		return fmt.Errorf("블록 스토리지 타입 선택 실패: %w", err)
	}
	if err := page.Locator("xpath=" + blockSizeXPath).Fill(blockSize); err != nil {
		return fmt.Errorf("블록 스토리지 용량 입력 실패: %w", err)
	}
	if err := page.Locator("xpath=" + blockNameXPath).Fill(blockName); err != nil {
		return fmt.Errorf("블록 스토리지 이름 입력 실패: %w", err)
	}
	if err := page.Locator("xpath=" + blockAddXPath).Click(); err != nil {
		return fmt.Errorf("블록 스토리지 추가 버튼 클릭 실패: %w", err)
	}

	page.WaitForTimeout(150)
	return nil
}

func configureFirewall(page playwright.Page, firewallName string) {
	if firewallName == "" {
		noFirewall := page.Locator("label:has-text('사용안함'), label:has-text('기본 방화벽')").First()
		if count, _ := noFirewall.Count(); count > 0 {
			_ = noFirewall.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(2000)})
		}
		return
	}

	if err := page.Locator("xpath=" + firewallEnableXPath).Click(); err != nil {
		fmt.Printf("⚠️ 경고: 방화벽 사용 설정 UI를 열지 못했습니다. (%v)\n", err)
		return
	}
	if err := waitForXPathVisible(page, "방화벽 목록", firewallListXPath, 3*time.Second); err != nil {
		fmt.Printf("⚠️ 경고: 방화벽 목록 준비 지연으로 선택을 건너뜁니다. (%v)\n", err)
		return
	}

	found := false
	for i := 0; i < 5; i++ {
		text, _ := page.Locator("xpath=" + firewallListXPath).InnerText()
		if strings.Contains(text, firewallName) {
			selector := fmt.Sprintf("xpath=%s//label[contains(., '%s')]//input", firewallListXPath, firewallName)
			if err := page.Locator(selector).Click(); err == nil {
				found = true
				break
			}
		}

		nextButton := page.Locator("xpath=" + firewallNextXPath).Last()
		if disabled, _ := nextButton.IsDisabled(); disabled {
			break
		}

		_ = nextButton.Click()
		page.WaitForTimeout(180)
	}

	if !found {
		fmt.Printf("⚠️ 경고: '%s' 방화벽을 찾지 못했습니다.\n", firewallName)
	}
}
