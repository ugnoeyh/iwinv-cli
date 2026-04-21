package console

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"iwinv-cli/internal/domain"
	"iwinv-cli/internal/ui"

	"github.com/playwright-community/playwright-go"
)

type trafficDailyUsage struct {
	Date              string
	TotalText         string
	InternationalText string
	TotalMB           float64
	InternationalMB   float64
}

func RunShowServerTraffic(page playwright.Page, target string, year int, month int) error {
	target = strings.TrimSpace(target)
	if target == "" {
		fmt.Println("ℹ️ 서버 IDX를 모르시면 아래 목록의 IP/IDX를 참고하세요.")
		if err := RunListServers(page); err != nil {
			return err
		}
		target = strings.TrimSpace(ui.PromptLine("조회할 서버 IDX 또는 이름: "))
		if target == "" {
			return fmt.Errorf("❌ 조회할 서버를 입력하세요 (--target)")
		}
	}

	selected, err := LookupServer(page, target)
	if err != nil {
		return err
	}

	year, month, err = resolveTrafficPeriod(year, month)
	if err != nil {
		return err
	}

	rows, err := fetchServerTrafficDailyUsage(page, selected.Idx, year, month)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Printf("ℹ️ [%s | IDX:%s] %04d-%02d 트래픽 데이터가 없습니다.\n", selected.Name, selected.Idx, year, month)
		return nil
	}

	printTrafficDailyUsage(selected, year, month, rows)
	return nil
}

func resolveTrafficPeriod(year int, month int) (int, int, error) {
	now := time.Now()

	if year == 0 {
		value, err := promptIntWithDefault(
			fmt.Sprintf("조회 연도 (YYYY, 기본 %d): ", now.Year()),
			now.Year(),
			func(v int) bool { return v >= 2000 && v <= 2100 },
			"연도는 2000~2100 범위로 입력하세요.",
		)
		if err != nil {
			return 0, 0, err
		}
		year = value
	}

	if month == 0 {
		value, err := promptIntWithDefault(
			fmt.Sprintf("조회 월 (1~12, 기본 %d): ", int(now.Month())),
			int(now.Month()),
			func(v int) bool { return v >= 1 && v <= 12 },
			"월은 1~12 범위로 입력하세요.",
		)
		if err != nil {
			return 0, 0, err
		}
		month = value
	}

	if year < 2000 || year > 2100 {
		return 0, 0, fmt.Errorf("❌ --traffic-year 값이 올바르지 않습니다: %d (허용 범위: 2000~2100)", year)
	}
	if month < 1 || month > 12 {
		return 0, 0, fmt.Errorf("❌ --traffic-month 값이 올바르지 않습니다: %d (허용 범위: 1~12)", month)
	}

	return year, month, nil
}

func promptIntWithDefault(label string, defaultVal int, validate func(int) bool, validationMsg string) (int, error) {
	for {
		raw := strings.TrimSpace(ui.PromptLine(label))
		if raw == "" {
			if validate == nil || validate(defaultVal) {
				return defaultVal, nil
			}
			return 0, fmt.Errorf("기본값이 유효하지 않습니다: %d", defaultVal)
		}

		value, err := strconv.Atoi(raw)
		if err != nil {
			fmt.Println("⚠️ 숫자로 입력해주세요.")
			continue
		}
		if validate != nil && !validate(value) {
			fmt.Printf("⚠️ %s\n", validationMsg)
			continue
		}
		return value, nil
	}
}

func fetchServerTrafficDailyUsage(page playwright.Page, idx string, year int, month int) ([]trafficDailyUsage, error) {
	raw, err := page.Evaluate(`async ([idx, year, month]) => {
		const monthText = String(month).padStart(2, "0");
		const url =
			"https://console.iwinv.kr/instance/tab/traffic?idx=" + encodeURIComponent(String(idx)) +
			"&unit=mb&year=" + encodeURIComponent(String(year)) +
			"&month=" + encodeURIComponent(monthText) +
			"&ajax=true";

		const res = await fetch(url, {
			method: "GET",
			headers: {
				"accept": "text/html, */*; q=0.01",
				"x-requested-with": "XMLHttpRequest"
			},
			credentials: "same-origin"
		});

		const html = await res.text();
		if (!res.ok) {
			return {
				status: res.status,
				error: "HTTP " + res.status,
				body: html.slice(0, 300)
			};
		}

		const normalizeWhitespace = (text) => (text || "").replace(/\s+/g, " ").trim();
		const parseTrafficCell = (cell) => {
			const raw = normalizeWhitespace((cell && (cell.innerText || cell.textContent)) || "");
			const compact = raw.replace(/,/g, "");
			const mbMatch = compact.match(/([0-9]+(?:\.[0-9]+)?)\s*MB/i);
			const gbMatch = compact.match(/([0-9]+(?:\.[0-9]+)?)\s*GB/i);

			const mbText = mbMatch ? mbMatch[1] + " MB" : "";
			const gbText = gbMatch ? gbMatch[1] + " GB" : "";
			let display = raw;
			if (mbText || gbText) {
				display = mbText && gbText ? (mbText + " (" + gbText + ")") : (mbText || gbText);
			}
			if (!display) {
				display = "-";
			}

			return {
				text: display,
				mb: mbMatch ? parseFloat(mbMatch[1]) : 0
			};
		};

		const doc = new DOMParser().parseFromString(html, "text/html");
		const rows = [];

		for (const tr of Array.from(doc.querySelectorAll("tr"))) {
			const cells = Array.from(tr.querySelectorAll("th,td"));
			if (cells.length < 3) continue;

			const dateText = normalizeWhitespace(cells[0].textContent || "");
			if (!dateText) continue;

			const normalizedDate = dateText.replace(/\s+/g, "");
			if (normalizedDate.includes("날짜")) continue;

			const total = parseTrafficCell(cells[1]);
			const international = parseTrafficCell(cells[2]);

			rows.push({
				date: dateText,
				totalText: total.text,
				internationalText: international.text,
				totalMB: Number.isFinite(total.mb) ? total.mb : 0,
				internationalMB: Number.isFinite(international.mb) ? international.mb : 0
			});
		}

		return { status: res.status, rows: rows };
	}`, []interface{}{idx, year, month})
	if err != nil {
		return nil, fmt.Errorf("트래픽 조회 스크립트 실행 오류: %w", err)
	}

	resMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("트래픽 조회 응답 형식을 해석할 수 없습니다")
	}

	status := toInt(resMap["status"])
	if status != 200 {
		errText := strings.TrimSpace(toString(resMap["error"]))
		bodyPreview := strings.TrimSpace(toString(resMap["body"]))
		if bodyPreview != "" {
			return nil, fmt.Errorf("트래픽 조회 실패 (status=%d, error=%s, body=%s)", status, errText, previewText(bodyPreview, 200))
		}
		return nil, fmt.Errorf("트래픽 조회 실패 (status=%d, error=%s)", status, errText)
	}

	rowsRaw, ok := resMap["rows"].([]interface{})
	if !ok {
		return nil, nil
	}

	rows := make([]trafficDailyUsage, 0, len(rowsRaw))
	for _, value := range rowsRaw {
		item, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		dateText := strings.TrimSpace(toString(item["date"]))
		if dateText == "" {
			continue
		}

		rows = append(rows, trafficDailyUsage{
			Date:              dateText,
			TotalText:         strings.TrimSpace(toString(item["totalText"])),
			InternationalText: strings.TrimSpace(toString(item["internationalText"])),
			TotalMB:           toFloat(item["totalMB"]),
			InternationalMB:   toFloat(item["internationalMB"]),
		})
	}

	return rows, nil
}

func printTrafficDailyUsage(selected *domain.ServerInfo, year int, month int, rows []trafficDailyUsage) {
	fmt.Println("\n=== [트래픽 일일 사용량 상세] ===")
	fmt.Printf("서버: %s | IP: %s | IDX: %s\n", selected.Name, selected.IP, selected.Idx)
	fmt.Printf("조회월: %04d-%02d (단위: MB)\n", year, month)
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("%-12s | %-20s | %-20s\n", "날짜", "전체트래픽", "국제트래픽")
	fmt.Println("---------------------------------------------------------------")

	var totalSumMB float64
	var internationalSumMB float64
	for _, row := range rows {
		totalSumMB += row.TotalMB
		internationalSumMB += row.InternationalMB
		fmt.Printf("%-12s | %-20s | %-20s\n", row.Date, row.TotalText, row.InternationalText)
	}

	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("합계         | %-20s | %-20s\n", formatTrafficTotal(totalSumMB), formatTrafficTotal(internationalSumMB))
	fmt.Printf("조회 건수     | %d일\n", len(rows))
	if totalSumMB > 0 {
		fmt.Printf("국제 비중     | %.1f%%\n", (internationalSumMB/totalSumMB)*100)
	}
	fmt.Println("===============================================================")
}

func formatTrafficTotal(totalMB float64) string {
	return fmt.Sprintf("%s (%.2f GB)", formatTrafficMB(totalMB), totalMB/1024.0)
}

func formatTrafficMB(value float64) string {
	rounded := math.Round(value)
	if math.Abs(value-rounded) < 1e-9 {
		return fmt.Sprintf("%.0f MB", rounded)
	}
	return fmt.Sprintf("%.1f MB", value)
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		s := strings.TrimSpace(strings.ReplaceAll(val, ",", ""))
		if s == "" {
			return 0
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}
