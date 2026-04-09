package console

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"iwinv-cli/internal/domain"
	"iwinv-cli/internal/ui"

	"github.com/playwright-community/playwright-go"
)

func RunListServers(page playwright.Page) error {
	fmt.Println("🚀 서버 목록을 갱신 중입니다...")

	servers, err := getServers(page)
	if err != nil {
		return err
	}

	if len(servers) == 0 {
		fmt.Println("❌ 계정에 보유 중인 서버가 없습니다.")
		return nil
	}

	fmt.Println("\n=== [내 서버 목록] ===")
	for i, srv := range servers {
		fmt.Printf("[%d] %-20s | IP: %-15s | IDX: %s\n", i+1, srv.Name, srv.IP, srv.Idx)
	}
	fmt.Println("======================")
	return nil
}

func RunServerAction(page playwright.Page, actionType, state, target string) error {
	selected, err := lookupServer(page, target)
	if err != nil {
		return err
	}

	stateLower := strings.ToLower(state)
	var script string
	var scriptArgs []interface{}
	var actionDesc string

	switch actionType {
	case "power":
		if stateLower != "on" && stateLower != "off" {
			return fmt.Errorf("❌ 전원 상태는 'on' 또는 'off'만 가능합니다")
		}

		script = `async ([idx, act]) => {
			let csrf = document.querySelector('meta[name="csrf-token"]')?.content || "";
			let res = await fetch("https://console.iwinv.kr/instance/" + act + "?idx=" + idx, {
				method: "GET",
				headers: {
					"accept": "application/json",
					"x-csrf-token": csrf,
					"x-requested-with": "XMLHttpRequest"
				}
			});
			return { status: res.status, body: await res.text() };
		}`
		scriptArgs = []interface{}{selected.Idx, stateLower}
		actionDesc = fmt.Sprintf("전원 %s", strings.ToUpper(stateLower))

	case "ip":
		if stateLower != "on" && stateLower != "off" {
			return fmt.Errorf("❌ IP 상태는 'on' 또는 'off'만 가능합니다")
		}

		active := "Y"
		if stateLower == "off" {
			active = "N"
		}

		script = `async ([idx, active]) => {
			let csrf = document.querySelector('meta[name="csrf-token"]')?.content || "";
			let res = await fetch("https://console.iwinv.kr/monitor/ip", {
				method: "POST",
				headers: {
					"content-type": "application/x-www-form-urlencoded; charset=UTF-8",
					"x-csrf-token": csrf,
					"x-requested-with": "XMLHttpRequest"
				},
				body: "idx=" + idx + "&active=" + active + "&_type=%7B%22active%22%3A%22radio%22%7D"
			});
			return { status: res.status, body: await res.text() };
		}`
		scriptArgs = []interface{}{selected.Idx, active}
		actionDesc = fmt.Sprintf("공인 IP %s", strings.ToUpper(stateLower))

	default:
		return fmt.Errorf("❌ 지원하지 않는 제어 타입입니다: %s", actionType)
	}

	result, err := page.Evaluate(script, scriptArgs)
	if err != nil {
		return fmt.Errorf("❌ 스크립트 실행 오류: %w", err)
	}

	resMap, ok := result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("❌ 제어 응답 형식을 해석할 수 없습니다")
	}

	if toInt(resMap["status"]) != 200 {
		return fmt.Errorf("❌ %s 요청 실패 (상태 코드: %v)", actionDesc, resMap["status"])
	}

	fmt.Printf("✅ [%s] %s 요청 성공!\n", selected.Name, actionDesc)
	time.Sleep(1 * time.Second)
	return nil
}

func RunDeleteServer(page playwright.Page, target string, pw string) error {
	selected, err := lookupServer(page, target)
	if err != nil {
		return err
	}

	fmt.Println("\n==================================================")
	fmt.Println("⚠️ [치명적 경고] 서버 삭제 작업이 시작됩니다!")
	fmt.Printf("▶ 대상 서버: %s\n", selected.Name)
	fmt.Printf("▶ IP 주소  : %s\n", selected.IP)
	fmt.Printf("▶ 고유 IDX : %s\n", selected.Idx)
	fmt.Println("==================================================")

	if !ui.ConfirmAction("❓ 이 서버를 정말로 영구 삭제하시겠습니까? (y/N): ") {
		fmt.Println("🛑 사용자 취소: 서버 삭제를 중단하고 안전하게 종료합니다.")
		return nil
	}

	fmt.Println("🗑️ 서버 삭제 API를 호출하는 중...")

	script := `async ([idx, pw]) => {
		let csrf = document.querySelector('meta[name="csrf-token"]')?.content || "";
		let bodyData = "active=Y&_token=" + encodeURIComponent(csrf) + "&pw=" + encodeURIComponent(pw) + "&_type=%7B%22pw%22%3A%22password%22%7D";

		let res = await fetch("https://console.iwinv.kr/instance/" + idx, {
			method: "POST",
			headers: {
				"accept": "application/json, text/javascript, */*; q=0.01",
				"content-type": "application/x-www-form-urlencoded; charset=UTF-8",
				"x-csrf-token": csrf,
				"x-requested-with": "XMLHttpRequest"
			},
			body: bodyData
		});
		return { status: res.status, body: await res.text() };
	}`

	result, err := page.Evaluate(script, []interface{}{selected.Idx, pw})
	if err != nil {
		return fmt.Errorf("❌ 삭제 스크립트 실행 오류: %w", err)
	}

	resMap, ok := result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("❌ 삭제 응답 형식을 해석할 수 없습니다")
	}

	if toInt(resMap["status"]) != 200 {
		return fmt.Errorf("❌ 서버 삭제 실패 (상태 코드: %v, 응답: %v)", resMap["status"], resMap["body"])
	}

	fmt.Printf("✅ [%s] 서버가 정상적으로 삭제되었습니다! (Goodbye~ 👋)\n", selected.Name)
	return nil
}

func getServers(page playwright.Page) ([]domain.ServerInfo, error) {
	if _, err := page.Goto(instanceURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return nil, fmt.Errorf("인스턴스 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "인스턴스 페이지 접속"); err != nil {
		return nil, err
	}

	page.WaitForTimeout(3000)

	elements := page.Locator("tr[data-idx]")
	count, err := elements.Count()
	if err != nil {
		return nil, fmt.Errorf("서버 목록 요소 카운트 실패: %w", err)
	}
	re := regexp.MustCompile(`(?:115\.68|49\.247)\.\d{1,3}\.\d{1,3}`)
	servers := make([]domain.ServerInfo, 0, count)

	for i := 0; i < count; i++ {
		el := elements.Nth(i)
		idx, err := el.GetAttribute("data-idx")
		if err != nil || idx == "" {
			continue
		}
		text, _ := el.InnerText()
		ips := re.FindAllString(text, -1)
		ipAddr := "OFF (미할당)"
		if len(ips) > 0 {
			ipAddr = ips[0]
		}

		nameLocator := el.Locator(".inline-flex.items-center.justify-center.gap-3").First()
		nameCount, _ := nameLocator.Count()
		name := ""
		if nameCount > 0 {
			name, _ = nameLocator.InnerText()
		} else {
			name, _ = el.Locator("td").First().InnerText()
		}

		name = strings.TrimSpace(strings.Split(name, "\n")[0])
		servers = append(servers, domain.ServerInfo{IP: ipAddr, Idx: idx, Name: name})
	}

	return servers, nil
}

func lookupServer(page playwright.Page, target string) (*domain.ServerInfo, error) {
	fmt.Printf("🚀 대상 서버('%s') 정보를 찾는 중...\n", target)

	servers, err := getServers(page)
	if err != nil {
		return nil, err
	}

	selected := findServer(servers, target)
	if selected == nil {
		return nil, fmt.Errorf("일치하는 서버('%s')를 찾을 수 없습니다", target)
	}

	return selected, nil
}

func findServer(servers []domain.ServerInfo, target string) *domain.ServerInfo {
	for i := range servers {
		if strings.Contains(servers[i].Name, target) || servers[i].Idx == target {
			return &servers[i]
		}
	}
	return nil
}
