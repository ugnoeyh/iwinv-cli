package console

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"iwinv-cli/internal/domain"
	"iwinv-cli/internal/ui"

	"github.com/playwright-community/playwright-go"
)

var ipv4LikeRegex = regexp.MustCompile(`^\d{1,3}(?:\.\d{1,3}){3}$`)

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
	selected, err := LookupServer(page, target)
	if err != nil {
		return err
	}
	return RunServerActionFor(page, selected, actionType, state)
}

func RunServerActionFor(page playwright.Page, selected *domain.ServerInfo, actionType, state string) error {
	if selected == nil {
		return fmt.Errorf("대상 서버 정보가 비어 있습니다")
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

	bodyText := strings.TrimSpace(toString(resMap["body"]))
	if bodyText != "" {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(bodyText), &obj); err == nil {
			if resultObj, ok := obj["result"].(map[string]interface{}); ok {
				code := toInt(resultObj["code"])
				message := strings.TrimSpace(toString(resultObj["message"]))
				if code != 0 {
					if message == "" {
						message = "알 수 없는 오류"
					}
					return fmt.Errorf("❌ 서버 삭제 실패 (code=%d, message=%s)", code, message)
				}
			} else {
				code := toInt(obj["code"])
				if code != 0 && obj["code"] != nil {
					message := strings.TrimSpace(toString(obj["message"]))
					if message == "" {
						message = "알 수 없는 오류"
					}
					return fmt.Errorf("❌ 서버 삭제 실패 (code=%d, message=%s)", code, message)
				}
			}
		} else {
			lower := strings.ToLower(bodyText)
			if strings.Contains(lower, "오류") ||
				strings.Contains(lower, "실패") ||
				strings.Contains(lower, "비밀번호") ||
				strings.Contains(lower, "password") {
				return fmt.Errorf("❌ 서버 삭제 실패 응답: %s", previewText(bodyText, 300))
			}
		}
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

	selected, err := findServer(servers, target)
	if err != nil {
		return nil, err
	}
	if selected == nil && isIPv4Like(target) {
		fallback, fbErr := lookupServerByIPFromPage(page, strings.TrimSpace(target))
		if fbErr != nil {
			return nil, fbErr
		}
		if fallback != nil {
			return fallback, nil
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("일치하는 서버('%s')를 찾을 수 없습니다", target)
	}

	return selected, nil
}

func LookupServer(page playwright.Page, target string) (*domain.ServerInfo, error) {
	return lookupServer(page, target)
}

func findServer(servers []domain.ServerInfo, target string) (*domain.ServerInfo, error) {
	target = strings.TrimSpace(target)
	targetLower := strings.ToLower(target)

	for i := range servers {
		if servers[i].Idx == target {
			return &servers[i], nil
		}
	}

	if isIPv4Like(target) {
		var ipExactMatches []*domain.ServerInfo
		for i := range servers {
			if strings.TrimSpace(servers[i].IP) == target {
				ipExactMatches = append(ipExactMatches, &servers[i])
			}
		}
		if len(ipExactMatches) == 1 {
			return ipExactMatches[0], nil
		}
		if len(ipExactMatches) > 1 {
			names := make([]string, 0, len(ipExactMatches))
			for _, m := range ipExactMatches {
				names = append(names, fmt.Sprintf("%s(IP:%s, IDX:%s)", m.Name, m.IP, m.Idx))
			}
			return nil, fmt.Errorf("'%s' IP에 매칭되는 서버가 여러 개입니다: %s\n정확한 서버 IDX를 지정하세요", target, strings.Join(names, " | "))
		}
	}

	var exactNameMatches []*domain.ServerInfo
	var matches []*domain.ServerInfo
	for i := range servers {
		nameLower := strings.ToLower(strings.TrimSpace(servers[i].Name))
		ipLower := strings.ToLower(strings.TrimSpace(servers[i].IP))
		if nameLower == targetLower {
			exactNameMatches = append(exactNameMatches, &servers[i])
			continue
		}
		if strings.Contains(nameLower, targetLower) || strings.Contains(ipLower, targetLower) {
			matches = append(matches, &servers[i])
		}
	}

	if len(exactNameMatches) == 1 {
		return exactNameMatches[0], nil
	}
	if len(exactNameMatches) > 1 {
		matches = exactNameMatches
	}

	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%s(IDX:%s)", m.Name, m.Idx))
		}
		return nil, fmt.Errorf("'%s'에 매칭되는 서버가 여러 개입니다: %s\n정확한 서버 IDX 또는 이름을 지정하세요", target, strings.Join(names, " | "))
	}
	return matches[0], nil
}

func lookupServerByIPFromPage(page playwright.Page, targetIP string) (*domain.ServerInfo, error) {
	raw, err := page.Evaluate(`(targetIP) => {
		function normalize(text) {
			return (text || "").replace(/\s+/g, " ").trim();
		}

		const rows = Array.from(document.querySelectorAll("tr[data-idx]"));
		const matches = [];

		for (const row of rows) {
			const text = normalize(row.innerText || row.textContent || "");
			if (!text.includes(targetIP)) continue;

			const idx = (row.getAttribute("data-idx") || "").trim();
			if (!idx) continue;

			let name = "";
			const nameNode = row.querySelector(".inline-flex.items-center.justify-center.gap-3") || row.querySelector("td");
			if (nameNode) {
				name = normalize(nameNode.innerText || nameNode.textContent || "");
				if (name.includes("\n")) {
					name = normalize(name.split("\n")[0]);
				}
			}

			matches.push({ idx, name });
		}

		return { matches };
	}`, targetIP)
	if err != nil {
		return nil, fmt.Errorf("IP fallback 조회 스크립트 실행 오류: %w", err)
	}

	resMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("IP fallback 조회 결과를 해석할 수 없습니다")
	}

	items, ok := resMap["matches"].([]interface{})
	if !ok || len(items) == 0 {
		return nil, nil
	}

	if len(items) > 1 {
		names := make([]string, 0, len(items))
		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			name := strings.TrimSpace(toString(m["name"]))
			idx := strings.TrimSpace(toString(m["idx"]))
			names = append(names, fmt.Sprintf("%s(IDX:%s)", name, idx))
		}
		return nil, fmt.Errorf("'%s' IP에 매칭되는 서버가 여러 개입니다: %s\n정확한 서버 IDX를 지정하세요", targetIP, strings.Join(names, " | "))
	}

	item, ok := items[0].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	return &domain.ServerInfo{
		IP:   targetIP,
		Idx:  strings.TrimSpace(toString(item["idx"])),
		Name: strings.TrimSpace(toString(item["name"])),
	}, nil
}

func isIPv4Like(value string) bool {
	return ipv4LikeRegex.MatchString(strings.TrimSpace(value))
}
