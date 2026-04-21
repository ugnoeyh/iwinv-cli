# TOOLS.md

## iwinvctl (Linux OpenClaw 전용 실행 가이드)

이 문서는 **Linux OpenClaw 환경**에서 `iwinvctl`를 안전하게 실행하기 위한 운영 기준입니다.

## 1) 고정 작업 경로
- 작업 디렉터리: `/root/iwinv`
- 실행 바이너리: `./bin/iwinvctl`
- 환경파일: `./.env`
- 세션파일: `./state.json` (현재 작업 디렉터리 기준)

## 2) 기본 실행 템플릿
모든 명령은 아래 템플릿을 기준으로 실행합니다.

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl <flags...>
```

## 3) 빌드
Linux amd64:
```bash
cd /root/iwinv && mkdir -p bin && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/iwinvctl ./cmd/iwinvctl
```

검증:
```bash
cd /root/iwinv && ./bin/iwinvctl --list
```

## 4) 로그인/세션 규칙
- `state.json`이 있으면 저장 세션을 재사용합니다.
- 세션이 만료되었거나 꼬였으면 아래로 재로그인합니다.

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --login
```

주의:
- 최초 로그인 시 ID/PW 입력이 필요합니다.
- 비대화형 환경에서는 로그인 프롬프트 입력이 불가하므로, `state.json`을 먼저 만들어 둬야 합니다.

## 5) 보안 규칙
- 비밀번호 하드코딩 금지
- `.env`에 `IWINV_PW` 저장 후 사용
- `.env`, `state.json`, 키/인증서 파일은 절대 공개 저장소 커밋 금지

예시 `.env`:
```env
IWINV_ID=your-id
IWINV_PW=your-password
```

## 6) 액션별 명령어

서버 조회:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --list
```

서버 트래픽 조회(일일 상세):
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --traffic --target "<서버명 또는 IDX>"
cd /root/iwinv && source .env && ./bin/iwinvctl --traffic --target "<서버명 또는 IDX>" --traffic-year 2026 --traffic-month 3
```

요금 상세 조회(서버/상품별):
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --bill
cd /root/iwinv && source .env && ./bin/iwinvctl --bill --bill-year 2026 --bill-month 4
cd /root/iwinv && source .env && ./bin/iwinvctl --bill --target "115.68.249.73"
```

전원 제어:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --power on --target "<서버명 또는 IDX>"
cd /root/iwinv && source .env && ./bin/iwinvctl --power off --target "<서버명 또는 IDX>"
```

공인 IP 제어:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --ip on --target "<서버명 또는 IDX>"
cd /root/iwinv && source .env && ./bin/iwinvctl --ip off --target "<서버명 또는 IDX>"
```

서버 삭제:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --delete --target "<서버명 또는 IDX>"
```

생성용 메타 조회:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --create-region1
cd /root/iwinv && source .env && ./bin/iwinvctl --create-region2 "KR1-ZONE"
cd /root/iwinv && source .env && ./bin/iwinvctl --create-spec --region1 "KR1-ZONE" --region2 "KR1-Z02"
cd /root/iwinv && source .env && ./bin/iwinvctl --create-os --region1 "KR1-ZONE" --region2 "KR1-Z02" --spec "hpa_2.4-g2"
cd /root/iwinv && source .env && ./bin/iwinvctl --search-spec "2 vCore"
```

서버 생성:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --create \
  --region1 "KR1-ZONE" \
  --region2 "KR1-Z02" \
  --spec "hpa_2.4-g2" \
  --os-type "기본" \
  --os "Ubuntu 22.04" \
  --name "auto-server"
```

방화벽 목록/탭 조회:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-list
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-create --firewall "my-firewall"
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-tab inbound --firewall-ref "8761"
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-tab outbound --firewall-ref "8761"
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-tab international --firewall-ref "8761"
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-tab bot --firewall-ref "8761"
```

방화벽 룰 추가/삭제:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-add --firewall-ref "8761" --firewall-dir inbound --rule-protocol TCP --rule-port 9998 --rule-ip "115.68.248.221"
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-remove --firewall-ref "8761" --firewall-dir inbound --rule-protocol TCP --rule-port 9998 --rule-ip "115.68.248.221"
```

국제망/검색봇 정책:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-international "FOREIGN"
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-international-remove "CHINA,USA"
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-international-clear
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-bot-remove "NAVER"
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-bot-clear
```

서버별 방화벽 사용 설정:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-choice-server "296561" --firewall-choice-policy "10863" --firewall-choice-use "Y"
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-choice-server "296561" --firewall-choice-policy "10863" --firewall-choice-use "N"
```

기술지원 작성 페이지 바로 열기(동의 자동 처리):
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --support-write
```

## 7) 필수 인자/충돌 규칙
- `--power`, `--ip`, `--delete`는 `--target` 필수
- `--traffic-year`, `--traffic-month`는 `--traffic`와 함께 사용
- `--bill-year`, `--bill-month`는 `--bill`과 함께 사용
- `--delete`는 비밀번호 필수 (`IWINV_PW` 또는 `--pw`)
- `--create`는 `--region1`, `--region2`, `--spec` 필수
- `--qty >= 2`이면 비밀번호 필수
- `--firewall-create`는 `--firewall`(정책명) 필수
- `--firewall-tab`, `--firewall-add`, `--firewall-remove`, `--firewall-international*`, `--firewall-bot*`는 `--firewall-ref` 필수
- `--firewall-choice-use` 사용 시 `--firewall-choice-server`, `--firewall-choice-policy` 필수

동시 사용 불가:
- `--firewall-international` + `--firewall-international-remove`
- `--firewall-international` + `--firewall-international-clear`
- `--firewall-international-remove` + `--firewall-international-clear`
- `--firewall-bot` + `--firewall-bot-remove`
- `--firewall-bot` + `--firewall-bot-clear`
- `--firewall-bot-remove` + `--firewall-bot-clear`

## 8) 운영 안전 체크리스트
- 파괴적 작업(삭제/정책 변경) 전 `--list` 또는 정책 조회로 대상 확인
- 이름 부분매칭(`--target`) 사용 시 동명 서버 충돌 여부 확인 후 진행
- 콘솔 DOM이 바뀌면 `internal/console/constants.go`, `internal/console/options.go` 우선 점검

## 9) 참고 파일
- 진입점: `cmd/iwinvctl/main.go`
- 플래그/분기: `internal/cli/flags.go`, `internal/cli/run.go`
- 로그인/세션: `internal/runtime/runtime.go`
- 서버 제어: `internal/console/servers.go`
- 조회/검색: `internal/console/query.go`
- 생성: `internal/console/create.go`
- 방화벽: `internal/console/firewall.go`, `internal/console/firewall_choice.go`
