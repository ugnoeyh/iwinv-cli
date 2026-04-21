package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"iwinv-cli/internal/console"
	"iwinv-cli/internal/envfile"
	"iwinv-cli/internal/runtime"

	"github.com/playwright-community/playwright-go"
)

func Execute() error {
	if err := checkDeprecatedFlags(os.Args[1:]); err != nil {
		return err
	}

	cfg := parseFlags()
	cfg.ApplyEnvDefaults()

	if !cfg.HasAction() {
		printUsage()
		return nil
	}

	headed := false
	appRuntime, err := runtime.New(cfg.Login, headed)
	if err != nil {
		return err
	}
	defer appRuntime.Close()

	if cfg.IsLoginOnly() {
		return nil
	}

	page, err := appRuntime.NewPage()
	if err != nil {
		return fmt.Errorf("페이지 생성 실패: %w", err)
	}
	defer page.Close()

	if err := runRequestedActions(page, cfg); err != nil {
		return err
	}

	if err := appRuntime.SaveState(); err != nil {
		log.Printf("세션 저장 실패: %v", err)
	}

	return nil
}

func runRequestedActions(page playwright.Page, cfg Flags) error {
	if cfg.FirewallDebug && cfg.hasFirewallAction() {
		printFirewallDebugBinaryFingerprint()
	}

	if cfg.FirewallInternational != "" && cfg.FirewallInternationalClear {
		return fmt.Errorf("❌ --firewall-international 과 --firewall-international-clear 는 동시에 사용할 수 없습니다")
	}
	if cfg.FirewallInternational != "" && cfg.FirewallInternationalRemove != "" {
		return fmt.Errorf("❌ --firewall-international 과 --firewall-international-remove 는 동시에 사용할 수 없습니다")
	}
	if cfg.FirewallInternationalRemove != "" && cfg.FirewallInternationalClear {
		return fmt.Errorf("❌ --firewall-international-remove 와 --firewall-international-clear 는 동시에 사용할 수 없습니다")
	}
	if cfg.FirewallBot != "" && cfg.FirewallBotClear {
		return fmt.Errorf("❌ --firewall-bot 과 --firewall-bot-clear 는 동시에 사용할 수 없습니다")
	}
	if cfg.FirewallBot != "" && cfg.FirewallBotRemove != "" {
		return fmt.Errorf("❌ --firewall-bot 과 --firewall-bot-remove 는 동시에 사용할 수 없습니다")
	}
	if cfg.FirewallBotRemove != "" && cfg.FirewallBotClear {
		return fmt.Errorf("❌ --firewall-bot-remove 와 --firewall-bot-clear 는 동시에 사용할 수 없습니다")
	}
	if cfg.FirewallChoiceUse != "" {
		if cfg.FirewallChoiceServer == "" {
			return fmt.Errorf("❌ 인스턴스 방화벽 사용 설정 시 --firewall-choice-server(서버 IDX 또는 이름)가 필요합니다")
		}
		if cfg.FirewallChoicePolicy == "" {
			return fmt.Errorf("❌ 인스턴스 방화벽 사용 설정 시 --firewall-choice-policy(방화벽 IDX 또는 이름)가 필요합니다")
		}
	}
	if !cfg.Traffic && (cfg.TrafficYear != 0 || cfg.TrafficMonth != 0) {
		return fmt.Errorf("❌ --traffic-year, --traffic-month 는 --traffic 와 함께 사용하세요")
	}
	if !cfg.Bill && (cfg.BillYear != 0 || cfg.BillMonth != 0) {
		return fmt.Errorf("❌ --bill-year, --bill-month 는 --bill 과 함께 사용하세요")
	}

	if cfg.SupportWrite {
		if err := console.RunOpenSupportRequestWrite(page); err != nil {
			return err
		}
	}

	if cfg.List {
		if err := console.RunListServers(page); err != nil {
			return err
		}
	}

	if cfg.Traffic {
		if err := console.RunShowServerTraffic(page, cfg.Target, cfg.TrafficYear, cfg.TrafficMonth); err != nil {
			return err
		}
	}

	if cfg.Bill {
		if err := console.RunShowBilling(page, cfg.Target, cfg.BillYear, cfg.BillMonth); err != nil {
			return err
		}
	}

	if cfg.Power != "" || cfg.IP != "" {
		if err := requireTarget(cfg.Target, "제어"); err != nil {
			return err
		}
		selected, err := console.LookupServer(page, cfg.Target)
		if err != nil {
			return err
		}

		if cfg.Power != "" {
			if err := console.RunServerActionFor(page, selected, "power", cfg.Power); err != nil {
				return err
			}
		}
		if cfg.IP != "" {
			if err := console.RunServerActionFor(page, selected, "ip", cfg.IP); err != nil {
				return err
			}
		}
	}

	if cfg.Delete {
		if err := requireTarget(cfg.Target, "삭제"); err != nil {
			return err
		}
		if cfg.Password == "" {
			return fmt.Errorf("❌ 서버 삭제 시 계정 비밀번호가 필요합니다. .env 파일에 %s를 설정하거나 --pw 옵션을 사용하세요", envfile.PasswordKey)
		}
		if err := console.RunDeleteServer(page, cfg.Target, cfg.Password); err != nil {
			return err
		}
	}

	if cfg.QueryRegion1 || cfg.QueryRegion2 != "" || cfg.QuerySpec || cfg.QueryOS {
		if err := console.RunCLIQuery(
			page,
			cfg.QueryRegion1,
			cfg.QueryRegion2,
			cfg.QuerySpec,
			cfg.QueryOS,
			cfg.Region1,
			cfg.Region2,
			cfg.Spec,
			cfg.OSType,
		); err != nil {
			return err
		}
	}

	if cfg.FirewallList {
		if err := console.RunListFirewallPolicies(page); err != nil {
			return err
		}
	}

	if cfg.FirewallCreate {
		if cfg.Firewall == "" {
			return fmt.Errorf("❌ 방화벽 정책 생성 시 --firewall(정책 이름)이 필요합니다")
		}
		if err := console.RunCreateFirewallPolicy(page, cfg.Firewall, cfg.FirewallDebug); err != nil {
			return err
		}
	}

	if cfg.FirewallTab != "" {
		if cfg.FirewallRef == "" {
			return fmt.Errorf("❌ 방화벽 탭 조회 시 --firewall-ref(정책 IDX 또는 이름)가 필요합니다")
		}
		if err := console.RunShowFirewallTab(page, cfg.FirewallTab, cfg.FirewallRef); err != nil {
			return err
		}
	}

	if cfg.FirewallAdd {
		if cfg.FirewallRef == "" {
			return fmt.Errorf("❌ 방화벽 룰 추가 시 --firewall-ref(정책 IDX 또는 이름)가 필요합니다")
		}
		if cfg.RuleIP == "" || cfg.RulePort == "" {
			return fmt.Errorf("❌ 방화벽 룰 추가 시 --rule-ip, --rule-port는 필수입니다")
		}
		if err := console.RunAddFirewallRule(
			page,
			cfg.FirewallRef,
			cfg.FirewallDir,
			cfg.RuleProtocol,
			cfg.RulePort,
			cfg.RuleIP,
			cfg.RuleTitle,
			cfg.RuleMemo,
			cfg.FirewallDebug,
		); err != nil {
			return err
		}
	}

	if cfg.FirewallRemove {
		if cfg.FirewallRef == "" {
			return fmt.Errorf("❌ 방화벽 룰 삭제 시 --firewall-ref(정책 IDX 또는 이름)가 필요합니다")
		}
		if cfg.RuleIP == "" || cfg.RulePort == "" {
			return fmt.Errorf("❌ 방화벽 룰 삭제 시 --rule-ip, --rule-port는 필수입니다")
		}
		if err := console.RunRemoveFirewallRule(
			page,
			cfg.FirewallRef,
			cfg.FirewallDir,
			cfg.RuleProtocol,
			cfg.RulePort,
			cfg.RuleIP,
			cfg.FirewallDebug,
		); err != nil {
			return err
		}
	}

	if cfg.FirewallInternational != "" {
		if cfg.FirewallRef == "" {
			return fmt.Errorf("❌ 국제망 정책 변경 시 --firewall-ref(정책 IDX 또는 이름)가 필요합니다")
		}
		if err := console.RunSetFirewallInternational(
			page,
			cfg.FirewallRef,
			cfg.FirewallInternational,
			cfg.FirewallDebug,
		); err != nil {
			return err
		}
	}

	if cfg.FirewallInternationalRemove != "" {
		if cfg.FirewallRef == "" {
			return fmt.Errorf("❌ 국제망 정책 개별 제거 시 --firewall-ref(정책 IDX 또는 이름)가 필요합니다")
		}
		if err := console.RunRemoveFirewallInternational(
			page,
			cfg.FirewallRef,
			cfg.FirewallInternationalRemove,
			cfg.FirewallDebug,
		); err != nil {
			return err
		}
	}

	if cfg.FirewallInternationalClear {
		if cfg.FirewallRef == "" {
			return fmt.Errorf("❌ 국제망 정책 전체 제거 시 --firewall-ref(정책 IDX 또는 이름)가 필요합니다")
		}
		if err := console.RunClearFirewallInternational(
			page,
			cfg.FirewallRef,
			cfg.FirewallDebug,
		); err != nil {
			return err
		}
	}

	if cfg.FirewallBot != "" {
		if cfg.FirewallRef == "" {
			return fmt.Errorf("❌ 검색봇 정책 변경 시 --firewall-ref(정책 IDX 또는 이름)가 필요합니다")
		}
		if err := console.RunSetFirewallBot(
			page,
			cfg.FirewallRef,
			cfg.FirewallBot,
			cfg.FirewallDebug,
		); err != nil {
			return err
		}
	}

	if cfg.FirewallBotRemove != "" {
		if cfg.FirewallRef == "" {
			return fmt.Errorf("❌ 검색봇 정책 개별 제거 시 --firewall-ref(정책 IDX 또는 이름)가 필요합니다")
		}
		if err := console.RunRemoveFirewallBot(
			page,
			cfg.FirewallRef,
			cfg.FirewallBotRemove,
			cfg.FirewallDebug,
		); err != nil {
			return err
		}
	}

	if cfg.FirewallBotClear {
		if cfg.FirewallRef == "" {
			return fmt.Errorf("❌ 검색봇 정책 전체 제거 시 --firewall-ref(정책 IDX 또는 이름)가 필요합니다")
		}
		if err := console.RunClearFirewallBot(
			page,
			cfg.FirewallRef,
			cfg.FirewallDebug,
		); err != nil {
			return err
		}
	}

	if cfg.FirewallChoiceUse != "" {
		if err := console.RunSetInstanceFirewallChoice(
			page,
			cfg.FirewallChoiceServer,
			cfg.FirewallChoicePolicy,
			cfg.FirewallChoiceUse,
			cfg.FirewallDebug,
		); err != nil {
			return err
		}
	}

	if cfg.SearchSpec != "" {
		if err := console.RunFindSpecRegion(page, cfg.SearchSpec); err != nil {
			return err
		}
	}

	if cfg.Create {
		if cfg.Region1 == "" || cfg.Region2 == "" || cfg.Spec == "" {
			return fmt.Errorf("❌ 서버 생성 시 --region1, --region2, --spec 플래그는 필수입니다")
		}
		if cfg.Qty < 1 {
			return fmt.Errorf("❌ --qty 값은 1 이상이어야 합니다 (입력값: %d)", cfg.Qty)
		}
		if err := console.RunCreate(
			page,
			cfg.Region1,
			cfg.Region2,
			cfg.Spec,
			cfg.Name,
			cfg.Qty,
			cfg.Password,
			cfg.OSType,
			cfg.OS,
			cfg.Block,
			cfg.Firewall,
		); err != nil {
			return err
		}
	}

	return nil
}

func printFirewallDebugBinaryFingerprint() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("🧪 FWDEBUG | binary=unknown err=%v\n", err)
		return
	}
	if abs, absErr := filepath.Abs(exe); absErr == nil {
		exe = abs
	}
	info, statErr := os.Stat(exe)
	if statErr != nil {
		fmt.Printf("🧪 FWDEBUG | binary=%s statErr=%v\n", exe, statErr)
		return
	}
	fmt.Printf("🧪 FWDEBUG | binary=%s mtime=%s size=%d\n", exe, info.ModTime().Format(time.RFC3339), info.Size())
}

func requireTarget(target, action string) error {
	if target == "" {
		return fmt.Errorf("❌ %s할 대상이 없습니다. --target 옵션을 지정하세요", action)
	}
	return nil
}

func checkDeprecatedFlags(args []string) error {
	for _, arg := range args {
		if arg == "--support-request-write" || strings.HasPrefix(arg, "--support-request-write=") {
			return fmt.Errorf("❌ --support-request-write 옵션은 제거되었습니다. --support-write 를 사용하세요")
		}
	}
	return nil
}
