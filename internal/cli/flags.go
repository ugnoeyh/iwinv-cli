package cli

import (
	"flag"
	"fmt"
	"os"

	"iwinv-cli/internal/envfile"
)

type Flags struct {
	QueryRegion1                bool
	QueryRegion2                string
	QuerySpec                   bool
	QueryOS                     bool
	SearchSpec                  string
	FirewallList                bool
	FirewallCreate              bool
	FirewallTab                 string
	FirewallRef                 string
	FirewallAdd                 bool
	FirewallRemove              bool
	FirewallDir                 string
	FirewallInternational       string
	FirewallInternationalRemove string
	FirewallInternationalClear  bool
	FirewallBot                 string
	FirewallBotRemove           string
	FirewallBotClear            bool
	FirewallChoiceUse           string
	FirewallChoiceServer        string
	FirewallChoicePolicy        string
	FirewallDebug               bool
	SupportWrite                bool
	RuleIP                      string
	RulePort                    string
	RuleProtocol                string
	RuleTitle                   string
	RuleMemo                    string

	List         bool
	Traffic      bool
	TrafficYear  int
	TrafficMonth int
	Bill         bool
	BillYear     int
	BillMonth    int
	Power        string
	IP           string
	Delete       bool
	Target       string

	Create   bool
	Region1  string
	Region2  string
	Spec     string
	OSType   string
	OS       string
	Name     string
	Qty      int
	Password string
	Block    string
	Firewall string

	Login bool
}

func parseFlags() Flags {
	var cfg Flags

	flag.BoolVar(&cfg.QueryRegion1, "create-region1", false, "리전 1차 목록 조회")
	flag.StringVar(&cfg.QueryRegion2, "create-region2", "", "특정 리전 1차에 대한 리전 2차 목록 조회")
	flag.BoolVar(&cfg.QuerySpec, "create-spec", false, "서버 스펙 목록 조회")
	flag.BoolVar(&cfg.QueryOS, "create-os", false, "선택 가능한 운영체제(OS) 목록 조회")
	flag.StringVar(&cfg.SearchSpec, "search-spec", "", "특정 스펙을 제공하는 리전을 초고속 역추적 검색")
	flag.BoolVar(&cfg.FirewallList, "firewall-list", false, "ELCAP 방화벽 정책 목록 조회")
	flag.BoolVar(&cfg.FirewallCreate, "firewall-create", false, "ELCAP 방화벽 정책 생성")
	flag.StringVar(&cfg.FirewallTab, "firewall-tab", "", "ELCAP 정책 탭 조회 (inbound|outbound|international|bot)")
	flag.StringVar(&cfg.FirewallRef, "firewall-ref", "", "ELCAP 정책 IDX 또는 이름")
	flag.BoolVar(&cfg.FirewallAdd, "firewall-add", false, "ELCAP 방화벽 룰 추가")
	flag.BoolVar(&cfg.FirewallRemove, "firewall-remove", false, "ELCAP 방화벽 룰 삭제")
	flag.StringVar(&cfg.FirewallDir, "firewall-dir", "inbound", "ELCAP 룰 방향 (inbound|outbound|both)")
	flag.StringVar(&cfg.FirewallInternational, "firewall-international", "", "ELCAP 국제망 차단 대상 설정 (FOREIGN 또는 TAIWAN,CHINA,PHILIPPINES,USA,JAPAN)")
	flag.StringVar(&cfg.FirewallInternationalRemove, "firewall-international-remove", "", "ELCAP 국제망 차단 대상 개별 제거 (예: TAIWAN 또는 CHINA,USA)")
	flag.BoolVar(&cfg.FirewallInternationalClear, "firewall-international-clear", false, "ELCAP 국제망 차단 정책 전체 제거")
	flag.StringVar(&cfg.FirewallBot, "firewall-bot", "", "ELCAP 검색봇 차단 대상 설정 (ALL 또는 GOOGLE,NAVER,DAUM)")
	flag.StringVar(&cfg.FirewallBotRemove, "firewall-bot-remove", "", "ELCAP 검색봇 차단 대상 개별 제거 (예: NAVER 또는 GOOGLE,DAUM)")
	flag.BoolVar(&cfg.FirewallBotClear, "firewall-bot-clear", false, "ELCAP 검색봇 차단 정책 전체 제거")
	flag.StringVar(&cfg.FirewallChoiceUse, "firewall-choice-use", "", "인스턴스 방화벽 사용 설정 (Y|N 또는 on|off)")
	flag.StringVar(&cfg.FirewallChoiceServer, "firewall-choice-server", "", "대상 서버 IDX 또는 이름")
	flag.StringVar(&cfg.FirewallChoicePolicy, "firewall-choice-policy", "", "적용할 방화벽 정책 IDX 또는 이름")
	flag.BoolVar(&cfg.FirewallDebug, "firewall-debug", false, "ELCAP 방화벽 룰 추가 디버그 로그 출력")
	flag.BoolVar(&cfg.SupportWrite, "support-write", false, "기술지원 작성 페이지로 이동 (동의 자동 처리, 백그라운드)")
	flag.StringVar(&cfg.RuleIP, "rule-ip", "", "추가할 룰 대상 IP/CIDR (예: 115.68.248.221 또는 115.68.0.0/16)")
	flag.StringVar(&cfg.RulePort, "rule-port", "", "추가할 룰 포트 (예: 9998)")
	flag.StringVar(&cfg.RuleProtocol, "rule-protocol", "TCP", "추가할 룰 프로토콜 (TCP|UDP)")
	flag.StringVar(&cfg.RuleTitle, "rule-title", "", "추가할 룰 제목 (기본: '<PROTOCOL> 직접입력')")
	flag.StringVar(&cfg.RuleMemo, "rule-memo", "", "추가할 룰 메모")

	flag.BoolVar(&cfg.List, "list", false, "내 서버 목록 조회")
	flag.BoolVar(&cfg.Traffic, "traffic", false, "서버 트래픽 일일 사용량 조회")
	flag.IntVar(&cfg.TrafficYear, "traffic-year", 0, "트래픽 조회 연도 (예: 2026, 미입력 시 프롬프트)")
	flag.IntVar(&cfg.TrafficMonth, "traffic-month", 0, "트래픽 조회 월 (1~12, 미입력 시 프롬프트)")
	flag.BoolVar(&cfg.Bill, "bill", false, "요금 상세(서버/상품별) 조회")
	flag.IntVar(&cfg.BillYear, "bill-year", 0, "요금 조회 연도 (예: 2026, 미입력 시 현재 연도)")
	flag.IntVar(&cfg.BillMonth, "bill-month", 0, "요금 조회 월 (1~12, 미입력 시 현재 월)")
	flag.StringVar(&cfg.Power, "power", "", "서버 전원 제어 (on/off)")
	flag.StringVar(&cfg.IP, "ip", "", "서버 공인 IP 제어 (on/off)")
	flag.BoolVar(&cfg.Delete, "delete", false, "서버 영구 삭제 실행")
	flag.StringVar(&cfg.Target, "target", "", "제어/삭제할 서버의 이름 또는 IDX (부분 매칭)")

	flag.BoolVar(&cfg.Create, "create", false, "서버 원샷 자동 생성 실행")
	flag.StringVar(&cfg.Region1, "region1", "", "리전 1차")
	flag.StringVar(&cfg.Region2, "region2", "", "리전 2차")
	flag.StringVar(&cfg.Spec, "spec", "", "서버 스펙")
	flag.StringVar(&cfg.OSType, "os-type", "기본", "운영체제 탭 선택 (기본, MY, 솔루션)")
	flag.StringVar(&cfg.OS, "os", "Ubuntu 22.04", "운영체제 이름")
	flag.StringVar(&cfg.Name, "name", "auto-server", "서버 이름")
	flag.IntVar(&cfg.Qty, "qty", 1, "생성 수량")
	flag.StringVar(&cfg.Password, "pw", "", "초기 비밀번호 (CLI 옵션)")
	flag.StringVar(&cfg.Block, "block", "", "블록 스토리지 (형식: '타입:용량:이름')")
	flag.StringVar(&cfg.Firewall, "firewall", "", "ELCAP 방화벽 정책 이름")

	flag.BoolVar(&cfg.Login, "login", false, "기존 세션을 초기화하고 새로 로그인")

	flag.Parse()
	return cfg
}

func (cfg *Flags) ApplyEnvDefaults() {
	envfile.LoadDefault()
	if cfg.Password == "" {
		cfg.Password = os.Getenv(envfile.PasswordKey)
	}
}

func (cfg Flags) HasAction() bool {
	return cfg.Login || cfg.hasNonLoginAction()
}

func (cfg Flags) IsLoginOnly() bool {
	return cfg.Login && !cfg.hasNonLoginAction()
}

func (cfg Flags) hasNonLoginAction() bool {
	return cfg.QueryRegion1 ||
		cfg.QueryRegion2 != "" ||
		cfg.QuerySpec ||
		cfg.QueryOS ||
		cfg.SearchSpec != "" ||
		cfg.FirewallList ||
		cfg.FirewallCreate ||
		cfg.FirewallTab != "" ||
		cfg.FirewallAdd ||
		cfg.FirewallRemove ||
		cfg.FirewallInternational != "" ||
		cfg.FirewallInternationalRemove != "" ||
		cfg.FirewallInternationalClear ||
		cfg.FirewallBot != "" ||
		cfg.FirewallBotRemove != "" ||
		cfg.FirewallBotClear ||
		cfg.FirewallChoiceUse != "" ||
		cfg.SupportWrite ||
		cfg.Create ||
		cfg.List ||
		cfg.Traffic ||
		cfg.Bill ||
		cfg.Power != "" ||
		cfg.IP != "" ||
		cfg.Delete
}

func (cfg Flags) hasFirewallAction() bool {
	return cfg.FirewallList ||
		cfg.FirewallCreate ||
		cfg.FirewallTab != "" ||
		cfg.FirewallAdd ||
		cfg.FirewallRemove ||
		cfg.FirewallInternational != "" ||
		cfg.FirewallInternationalRemove != "" ||
		cfg.FirewallInternationalClear ||
		cfg.FirewallBot != "" ||
		cfg.FirewallBotRemove != "" ||
		cfg.FirewallBotClear ||
		cfg.FirewallChoiceUse != ""
}

func printUsage() {
	fmt.Println("⚠️ 실행 옵션이 지정되지 않았습니다.")
	flag.PrintDefaults()
}
