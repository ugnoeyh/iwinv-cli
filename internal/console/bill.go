package console

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

type billingCard struct {
	Title            string
	Badges           []string
	HeaderAmountText string
	HeaderAmountKRW  int64
	Details          []billingDetail
}

type billingDetail struct {
	Category   string
	Item       string
	AmountText string
	AmountKRW  int64
	StartAt    string
	EndAt      string
	Note       string
}

func RunShowBilling(page playwright.Page, target string, year int, month int) error {
	if _, err := page.Goto(financeBillURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("요금 페이지 접속 실패: %w", err)
	}
	if err := ensureAuthenticated(page, "요금 페이지 접속"); err != nil {
		return err
	}

	year, month, err := normalizeBillPeriod(year, month)
	if err != nil {
		return err
	}

	cards, err := fetchBillingCards(page, year, month)
	if err != nil {
		return err
	}
	if len(cards) == 0 {
		return fmt.Errorf("요금 상세 카드가 없습니다 (조회월: %04d-%02d)", year, month)
	}

	filtered := filterBillingCards(cards, target)
	if len(filtered) == 0 {
		return fmt.Errorf("요금 항목에서 '%s'에 매칭되는 결과가 없습니다", target)
	}

	printBillingCards(filtered, year, month, target)
	return nil
}

func normalizeBillPeriod(year int, month int) (int, int, error) {
	now := time.Now()
	if year == 0 {
		year = now.Year()
	}
	if month == 0 {
		month = int(now.Month())
	}

	if year < 2000 || year > 2100 {
		return 0, 0, fmt.Errorf("❌ --bill-year 값이 올바르지 않습니다: %d (허용 범위: 2000~2100)", year)
	}
	if month < 1 || month > 12 {
		return 0, 0, fmt.Errorf("❌ --bill-month 값이 올바르지 않습니다: %d (허용 범위: 1~12)", month)
	}

	return year, month, nil
}

func fetchBillingCards(page playwright.Page, year int, month int) ([]billingCard, error) {
	raw, err := page.Evaluate(`async ([year, month]) => {
		const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
		const normalize = (text) => (text || "").replace(/\s+/g, " ").trim();
		const parseKRW = (text) => {
			const cleaned = (text || "").replace(/,/g, "").replace(/[^0-9-]/g, "");
			if (!cleaned || cleaned === "-") return 0;
			const n = Number(cleaned);
			return Number.isFinite(n) ? n : 0;
		};

		const setField = (selectors, value) => {
			for (const selector of selectors) {
				const el = document.querySelector(selector);
				if (!el) continue;

				el.value = String(value);
				el.dispatchEvent(new Event("input", { bubbles: true }));
				el.dispatchEvent(new Event("change", { bubbles: true }));
				return true;
			}
			return false;
		};

		const clickSearch = () => {
			const nodes = Array.from(document.querySelectorAll("button, a, input[type='button'], input[type='submit']"));
			for (const node of nodes) {
				const text = normalize(node.textContent || node.value || "");
				if (text === "검색" || text.toLowerCase() === "search") {
					node.click();
					return true;
				}
			}
			return false;
		};

		const yearChanged = setField(["select[name='year']", "input[name='year']", "#year"], year);
		const monthChanged = setField(["select[name='month']", "input[name='month']", "#month"], month);
		const searchClicked = clickSearch();
		if (yearChanged || monthChanged || searchClicked) {
			// 카드가 나타날 때까지 폴링 (최대 3000ms)
			const deadline = Date.now() + 3000;
			while (Date.now() < deadline) {
				await delay(200);
				const root = document.querySelector("#tab-content-all") ||
					document.querySelector("[data-tab-content='all']") ||
					document;
				if (root.querySelectorAll(".grid-card").length > 0) break;
			}
		}

		const root =
			document.querySelector("#tab-content-all") ||
			document.querySelector("[data-tab-content='all']") ||
			document;

		const cards = [];
		const cardNodes = Array.from(root.querySelectorAll(".grid-card"));
		for (const card of cardNodes) {
			const title = normalize(card.querySelector("h3")?.textContent || "");
			if (!title) continue;

			const header = card.querySelector(".flex.items-center.justify-between.cursor-pointer") || card.querySelector(".flex.items-center.justify-between");
			let amountText = "";
			if (header) {
				const spans = Array.from(header.querySelectorAll("span"));
				for (const span of spans) {
					const text = normalize(span.textContent || "");
					if (text.includes("원")) {
						amountText = text;
					}
				}
			}

			const badges = Array.from(card.querySelectorAll(".flex.gap-2 span"))
				.map((node) => normalize(node.textContent || ""))
				.filter((text) => text && !text.includes("원"));

			const details = [];
			for (const tr of Array.from(card.querySelectorAll("table tbody tr"))) {
				const cols = Array.from(tr.querySelectorAll("td,th")).map((td) => normalize(td.textContent || ""));
				if (cols.length < 3) continue;
				details.push({
					category: cols[0] || "",
					item: cols[1] || "",
					amountText: cols[2] || "",
					amountKRW: parseKRW(cols[2] || ""),
					startAt: cols[3] || "",
					endAt: cols[4] || "",
					note: cols[5] || ""
				});
			}

			cards.push({
				title,
				badges,
				headerAmountText: amountText || "0 원",
				headerAmountKRW: parseKRW(amountText || ""),
				details
			});
		}

		return { cards };
	}`, []interface{}{year, month})
	if err != nil {
		return nil, fmt.Errorf("요금 상세 조회 스크립트 실행 오류: %w", err)
	}

	resMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("요금 상세 조회 응답 형식을 해석할 수 없습니다")
	}

	rawCards, ok := resMap["cards"].([]interface{})
	if !ok {
		return nil, nil
	}

	result := make([]billingCard, 0, len(rawCards))
	for _, item := range rawCards {
		cardMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		card := billingCard{
			Title:            strings.TrimSpace(toString(cardMap["title"])),
			HeaderAmountText: strings.TrimSpace(toString(cardMap["headerAmountText"])),
			HeaderAmountKRW:  int64(toInt(cardMap["headerAmountKRW"])),
		}
		if card.HeaderAmountText == "" {
			card.HeaderAmountText = formatKRW(card.HeaderAmountKRW)
		}

		if badgesRaw, ok := cardMap["badges"].([]interface{}); ok {
			for _, b := range badgesRaw {
				text := strings.TrimSpace(toString(b))
				if text != "" {
					card.Badges = append(card.Badges, text)
				}
			}
		}

		if detailsRaw, ok := cardMap["details"].([]interface{}); ok {
			for _, d := range detailsRaw {
				detailMap, ok := d.(map[string]interface{})
				if !ok {
					continue
				}
				detail := billingDetail{
					Category:   strings.TrimSpace(toString(detailMap["category"])),
					Item:       strings.TrimSpace(toString(detailMap["item"])),
					AmountText: strings.TrimSpace(toString(detailMap["amountText"])),
					AmountKRW:  int64(toInt(detailMap["amountKRW"])),
					StartAt:    strings.TrimSpace(toString(detailMap["startAt"])),
					EndAt:      strings.TrimSpace(toString(detailMap["endAt"])),
					Note:       strings.TrimSpace(toString(detailMap["note"])),
				}
				if detail.AmountText == "" {
					detail.AmountText = formatKRW(detail.AmountKRW)
				}
				card.Details = append(card.Details, detail)
			}
		}

		result = append(result, card)
	}

	return result, nil
}

func filterBillingCards(cards []billingCard, target string) []billingCard {
	target = strings.TrimSpace(target)
	if target == "" {
		return cards
	}

	targetLower := strings.ToLower(target)
	result := make([]billingCard, 0, len(cards))
	for _, card := range cards {
		if strings.Contains(strings.ToLower(card.Title), targetLower) {
			result = append(result, card)
			continue
		}

		matched := false
		for _, detail := range card.Details {
			if strings.Contains(strings.ToLower(detail.Item), targetLower) ||
				strings.Contains(strings.ToLower(detail.Category), targetLower) {
				matched = true
				break
			}
		}
		if matched {
			result = append(result, card)
		}
	}
	return result
}

func printBillingCards(cards []billingCard, year int, month int, target string) {
	header := fmt.Sprintf("요금 상세: %04d-%02d", year, month)
	if strings.TrimSpace(target) != "" {
		header += "  (필터: " + target + ")"
	}
	fmt.Println("\n" + header)
	fmt.Println(strings.Repeat("─", 52))

	var total int64
	for _, card := range cards {
		total += card.HeaderAmountKRW

		badges := ""
		if len(card.Badges) > 0 {
			badges = "  [" + strings.Join(card.Badges, " / ") + "]"
		}
		fmt.Printf("\n  %s%s\n", card.Title, badges)
		fmt.Printf("  %s\n", strings.Repeat("·", 48))

		if len(card.Details) == 0 {
			fmt.Printf("  %-36s  %s\n", "(상세 내역 없음)", formatKRW(card.HeaderAmountKRW))
			continue
		}

		for _, detail := range card.Details {
			label := strings.TrimSpace(detail.Item)
			if label == "" {
				label = strings.TrimSpace(detail.Category)
			}
			if label == "" {
				label = "-"
			}

			period := detailPeriod(detail)
			amtStr := normalizeAmountText(detail.AmountText, detail.AmountKRW)

			line := fmt.Sprintf("  %-34s  %10s", truncateText(label, 34), amtStr)
			if period != "-" {
				line += "   " + period
			}
			fmt.Println(line)

			if note := strings.TrimSpace(detail.Note); note != "" && note != "-" {
				fmt.Printf("    %s\n", note)
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("─", 52))
	fmt.Printf("합계: %s  (%d개)\n", formatKRW(total), len(cards))
}

func normalizeAmountText(text string, amount int64) string {
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return formatKRW(amount)
	}
	return cleaned
}


func detailPeriod(detail billingDetail) string {
	start := strings.TrimSpace(detail.StartAt)
	end := strings.TrimSpace(detail.EndAt)
	if start == "" && end == "" {
		return "-"
	}
	if start == "" {
		return end
	}
	if end == "" {
		return start
	}
	if start == end {
		return start
	}
	return start + " ~ " + end
}

func truncateText(v string, max int) string {
	text := strings.TrimSpace(v)
	if max <= 0 {
		return ""
	}

	r := []rune(text)
	if len(r) <= max {
		return text
	}
	if max == 1 {
		return string(r[:1])
	}
	return string(r[:max-1]) + "…"
}


func formatKRW(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}

	s := strconv.FormatInt(value, 10)
	if len(s) <= 3 {
		return sign + s + " 원"
	}

	var out []byte
	pre := len(s) % 3
	if pre > 0 {
		out = append(out, s[:pre]...)
		if len(s) > pre {
			out = append(out, ',')
		}
	}
	for i := pre; i < len(s); i += 3 {
		out = append(out, s[i:i+3]...)
		if i+3 < len(s) {
			out = append(out, ',')
		}
	}
	return sign + string(out) + " 원"
}
