# TOOLS.md

## 6. iwinvctl (iwinv 클라우드 인프라 관리 및 자동화 ☁️)

용도:
- iwinv 콘솔에 직접 들어가지 않고 터미널에서 서버 목록 조회
- 서버 전원 제어
- 공인 IP 제어
- 신규 서버 자동 생성
- 서버 삭제
- 생성 가능한 리전/스펙/OS 조회
- ELCAP 방화벽 정책 목록 조회
- ELCAP 정책 탭 조회(인바운드/아웃바운드/국제망/봇)
- ELCAP 인바운드/아웃바운드/양방향 룰 추가
- ELCAP 국제망/검색봇 차단 정책 설정/제거
- 서버별 ELCAP 방화벽 사용/해제

주의:
- 현재 바이너리는 예전 `-action=list` 스타일이 아니라 `--list`, `--power`, `--ip`, `--create` 같은 플래그 스타일을 사용한다.
- 현재 코드에는 `bill` 액션이 없다.
- 청구서/실시간 요금 조회는 아직 미구현이므로, OpenClaw는 이를 지원하는 것처럼 꾸며내면 안 된다.

---

## [🔥 iwinv 관리 절대 규칙]

작업 디렉토리는 무조건 `/root/iwinv` 기준이다.

실행 템플릿:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl <flags...>
```

보안 규칙:
- 비밀번호 하드코딩 금지
- 명령 실행 전 `source .env`를 먼저 걸어 환경변수를 로드한다
- `.env`에는 최소한 `IWINV_PW`가 들어갈 수 있다

제어 규칙:
- 사용자가 서버 이름만 말해도 되묻지 말고 그대로 `--target "<서버명>"`으로 실행해라
- 현재 바이너리는 `--target`에 이름 부분 일치 또는 IDX 완전 일치를 지원한다
- 다만 동일하거나 비슷한 이름이 여러 대일 가능성이 높으면 먼저 `--list`를 실행해서 IDX를 확인한 뒤 더 안전하게 진행해라

세션 규칙:
- 로그인 세션이 꼬였거나 만료된 것 같으면 먼저 아래 명령으로 세션을 재생성한다

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --login
```

---

## [✅ 액션별 명령어 가이드]

### 서버 목록 조회 (list)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --list
```

행동:
- 출력 결과에서 타겟 서버의 이름, IDX, 공인 IP 할당 여부를 파악할 때 사용
- 서버 제어 전에 충돌 가능성이 있으면 먼저 이 명령으로 확인

구현 위치:
- `internal/console/servers.go`

### 실시간 요금 조회 (bill)

현재 상태:
- 미지원

행동:
- 현재 코드베이스에는 `bill` 또는 청구서 조회 액션이 없다
- OpenClaw는 이 기능을 지원하는 것처럼 답하지 말고, "현재 바이너리에서는 미구현"이라고 말해야 한다
- 필요하면 별도 구현 작업이 필요하다

후보 구현 위치:
- `internal/console/` 아래 신규 파일 추가 필요

### 서버 전원 제어 (power)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --power on --target "<서버명 또는 IDX>"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --power off --target "<서버명 또는 IDX>"
```

행동:
- 사용자가 IDX를 안 줘도 이름으로 바로 실행 가능
- 이름 충돌이 의심되면 먼저 `--list` 후 IDX 기준으로 재실행

구현 위치:
- `internal/console/servers.go`

### 공인 IP 제어 (ip)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --ip on --target "<서버명 또는 IDX>"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --ip off --target "<서버명 또는 IDX>"
```

행동:
- 이름 기반 제어 가능
- 서버 현재 IP 할당 상태를 확인하고 싶으면 먼저 `--list`

구현 위치:
- `internal/console/servers.go`

### 서버 삭제 (delete)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --delete --target "<서버명 또는 IDX>"
```

또는:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --delete --target "<서버명 또는 IDX>" --pw "<비밀번호>"
```

행동:
- 삭제 전 확인 프롬프트가 뜬다
- `.env`에 `IWINV_PW`가 있으면 `--pw` 없이도 동작 가능

구현 위치:
- `internal/console/servers.go`

### ELCAP 방화벽 정책 목록 조회 (firewall-list)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-list
```

행동:
- ELCAP 방화벽 페이지(`https://console.iwinv.kr/firewall`)에서 정책 목록을 조회한다
- 각 정책의 이름과 IDX(`data-idx`)를 함께 출력한다

구현 위치:
- `internal/console/firewall.go`

### ELCAP 정책 탭 조회 (firewall-tab)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-tab inbound --firewall-ref "8761"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-tab outbound --firewall-ref "8761"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-tab international --firewall-ref "8761"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-tab bot --firewall-ref "8761"
```

행동:
- `--firewall-ref`는 정책 `IDX` 또는 정책명(부분 포함 매칭) 가능
- 조회 가능한 탭 값: `inbound`, `outbound`, `international`, `bot`
- 정책명 매칭이 여러 개면 충돌 방지를 위해 에러를 반환한다

구현 위치:
- `internal/console/firewall.go`

### ELCAP 룰 추가 (firewall-add)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-add --firewall-ref "8761" --firewall-dir inbound --rule-protocol TCP --rule-port 9998 --rule-ip "115.68.248.221"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-add --firewall-ref "8761" --firewall-dir outbound --rule-protocol TCP --rule-port 9998 --rule-ip "115.68.248.221"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-add --firewall-ref "8761" --firewall-dir both --rule-protocol TCP --rule-port 9998 --rule-ip "115.68.248.221"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-add --firewall-ref "8761" --firewall-dir inbound --rule-protocol TCP --rule-port 9998 --rule-ip "115.68.248.221" --firewall-debug
```

행동:
- 현재 정책 폼의 전체 값을 유지한 채 신규 룰 1건만 append하여 `/firewall` 저장 요청을 보낸다
- 룰 추가 후 해당 탭을 즉시 재조회해서 반영 여부를 확인한다
- 디버깅이 필요하면 `--firewall-debug`를 붙여 내부 단계 로그를 확인한다
- 지원 방향: `inbound`, `outbound`, `both`
- 지원 프로토콜: `TCP`, `UDP`

구현 위치:
- `internal/console/firewall.go`

### ELCAP 룰 삭제 (firewall-remove)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-remove --firewall-ref "8761" --firewall-dir inbound --rule-protocol TCP --rule-port 9998 --rule-ip "115.68.248.221"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-remove --firewall-ref "8761" --firewall-dir outbound --rule-protocol TCP --rule-port 9998 --rule-ip "115.68.248.221"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-remove --firewall-ref "8761" --firewall-dir both --rule-protocol TCP --rule-port 9998 --rule-ip "115.68.248.221"
```

행동:
- `--rule-protocol + --rule-port + --rule-ip`와 동일한 `unique` 룰을 찾아 제거한다
- 저장 후 해당 탭을 즉시 재조회해서 제거 반영 여부를 확인한다
- 지원 방향: `inbound`, `outbound`, `both`
- 지원 프로토콜: `TCP`, `UDP`

구현 위치:
- `internal/console/firewall.go`

### ELCAP 국제망 통신 정책 설정 (firewall-international)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-international "TAIWAN,CHINA,PHILIPPINES,USA,JAPAN"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-international "FOREIGN"
```

행동:
- `international[target][]` 값을 지정한 목록으로 재구성한 뒤 `/firewall`에 저장한다
- `FOREIGN`은 단독으로만 허용된다 (`FOREIGN` + 국가코드 조합 불가)
- 허용 국가코드: `TAIWAN`, `CHINA`, `PHILIPPINES`, `USA`, `JAPAN`
- 저장 후 `international` 탭과 폼 값을 재확인하여 반영을 검증한다

구현 위치:
- `internal/console/firewall.go`

### ELCAP 국제망 통신 정책 개별 제거 (firewall-international-remove)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-international-remove "TAIWAN" --firewall-debug
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-international-remove "CHINA,USA" --firewall-debug
```

행동:
- 현재 국제망 차단 목록을 조회한 뒤, 지정한 타깃만 제거하고 나머지는 유지한다
- 지정한 타깃이 현재 목록에 없으면 저장을 건너뛴다
- 저장 전 폼 스냅샷 안전검사를 수행해 기존 인바운드/아웃바운드 유실 위험이 있으면 중단한다
- 저장 후 `international` 탭을 재조회해 반영을 검증한다

구현 위치:
- `internal/console/firewall.go`

### ELCAP 국제망 통신 정책 전체 제거 (firewall-international-clear)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-international-clear --firewall-debug
```

행동:
- 국제망(`international`) 타깃을 0개로 저장하여 기존 국제망 차단 항목을 모두 제거한다
- 저장 전 폼 스냅샷 안전검사를 수행해 기존 인바운드/아웃바운드 유실 위험이 있으면 중단한다
- 저장 후 `international` 탭을 재조회해 비워졌는지 검증한다

구현 위치:
- `internal/console/firewall.go`

### ELCAP 검색봇 차단 정책 설정 (firewall-bot)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-bot "GOOGLE,NAVER"
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-bot "ALL"
```

행동:
- `bot[target][]` 값을 지정한 목록으로 재구성한 뒤 `/firewall`에 저장한다
- `ALL`은 단독으로만 허용된다 (`ALL` + 개별 봇 조합 불가)
- 허용 검색봇 코드: `GOOGLE`, `NAVER`, `DAUM`
- 저장 후 `bot` 탭과 폼 값을 재확인하여 반영을 검증한다

구현 위치:
- `internal/console/firewall.go`

### ELCAP 검색봇 차단 정책 개별 제거 (firewall-bot-remove)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-bot-remove "NAVER" --firewall-debug
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-bot-remove "GOOGLE,DAUM" --firewall-debug
```

행동:
- 현재 검색봇 차단 목록을 조회한 뒤, 지정한 타깃만 제거하고 나머지는 유지한다
- 지정한 타깃이 현재 목록에 없으면 저장을 건너뛴다
- 저장 전 폼 스냅샷 안전검사를 수행해 기존 인바운드/아웃바운드/국제망 유실 위험이 있으면 중단한다
- 저장 후 `bot` 탭을 재조회해 반영을 검증한다

구현 위치:
- `internal/console/firewall.go`

### ELCAP 검색봇 차단 정책 전체 제거 (firewall-bot-clear)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-ref "8761" --firewall-bot-clear --firewall-debug
```

행동:
- 검색봇(`bot`) 타깃을 0개로 저장하여 기존 검색봇 차단 항목을 모두 제거한다
- 저장 전 폼 스냅샷 안전검사를 수행해 기존 인바운드/아웃바운드/국제망 유실 위험이 있으면 중단한다
- 저장 후 `bot` 탭을 재조회해 비워졌는지 검증한다

구현 위치:
- `internal/console/firewall.go`

### 서버 방화벽 사용 설정 (firewall/choice)

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-choice-server "296561" --firewall-choice-policy "10863" --firewall-choice-use "Y" --firewall-debug
```

```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --firewall-choice-server "296561" --firewall-choice-policy "10863" --firewall-choice-use "N" --firewall-debug
```

행동:
- `POST /firewall/choice`로 서버의 방화벽 사용 상태를 변경한다
- `--firewall-choice-use`: `Y|N` 또는 `on|off`
- 서버/방화벽 식별자는 IDX 또는 이름(부분 매칭) 지원

구현 위치:
- `internal/console/firewall_choice.go`

---

## [🔥 서버 자동 생성 절대 규칙]

현재 바이너리 기준 필수 파라미터:
- `--create`
- `--region1`
- `--region2`
- `--spec`

현재 바이너리 기준 기본값:
- `--os-type "기본"`
- `--os "Ubuntu 22.04"`
- `--name "auto-server"`
- `--qty 1`

OpenClaw 실행 규칙:
- 사용자가 `region1`, `region2`, `spec`를 명시하지 않으면 OpenClaw는 실행 전 내부적으로 기본값을 가정해서라도 명령을 완성할 수 있다
- 권장 기본값:
  - `--region1 "KR-Seoul"`
  - `--region2 "Zone-A"`
  - `--os "Ubuntu 22.04"`
  - `--name "auto-server"`
- 단, `spec`은 사용 의도가 너무 넓어서 가능하면 사용자 요청 문맥에서 가장 가까운 값을 선택해야 한다

수량 및 비밀번호 규칙:
- `--qty`가 2 이상이면 `--pw`는 선택이 아니라 필수다
- 없으면 현재 코드에서 에러가 난다

블록 스토리지 규칙:
- 블록 추가 시 반드시 `종류:용량:이름` 포맷 사용
- 예시: `--block "SSD:200:test2"`

방화벽 규칙:
- 방화벽 이름은 정확히 일치하지 않아도 화면 목록 안에서 포함 문자열 기준으로 찾는다
- 그래도 못 찾으면 경고만 출력하고 계속 진행될 수 있다

### 서버 생성 기본형

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --create \
  --region1 "KR-Seoul" \
  --region2 "Zone-A" \
  --spec "2 vCPU 4 GB" \
  --os "Ubuntu 22.04" \
  --name "<서버명>"
```

### 서버 생성 풀옵션

실행:
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --create \
  --region1 "KR-Seoul" \
  --region2 "Zone-A" \
  --spec "2 vCPU 4 GB" \
  --os "Ubuntu 22.04" \
  --name "<서버명>" \
  --qty 2 \
  --pw "<비밀번호>" \
  --block "SSD:200:test" \
  --firewall "기본방화벽"
```

구현 위치:
- `internal/console/create.go`

---

## [✅ 생성 보조 조회 명령]

### 리전 1차 조회
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --create-region1
```

### 리전 2차 조회
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --create-region2 "KR-Seoul"
```

### 스펙 조회
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --create-spec --region1 "KR-Seoul" --region2 "Zone-A"
```

### OS 조회
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --create-os --region1 "KR-Seoul" --region2 "Zone-A" --spec "2 vCPU 4 GB"
```

### 특정 스펙 가능 리전 검색
```bash
cd /root/iwinv && source .env && ./bin/iwinvctl --search-spec "2 vCPU 4 GB"
```

구현 위치:
- `internal/console/query.go`

---

## [🔧 수정할 때 먼저 볼 파일]

진입점:
- `cmd/iwinvctl/main.go`

플래그/라우팅:
- `internal/cli/flags.go`
- `internal/cli/run.go`

로그인/세션:
- `internal/runtime/runtime.go`

서버 관리:
- `internal/console/servers.go`

조회 기능:
- `internal/console/query.go`

생성 기능:
- `internal/console/create.go`

공통 선택/XPath:
- `internal/console/options.go`
- `internal/console/constants.go`

출력 포맷:
- `internal/output/summary.go`

---

## [🧪 수정 후 검증 명령]

포맷:
```bash
gofmt -w $(find . -name '*.go' -not -path './.gocache/*')
```

테스트:
```bash
go test ./...
```

재빌드:
```bash
cd /root/iwinv && mkdir -p bin && go build -o ./bin/iwinvctl ./cmd/iwinvctl
```

---

## [📌 OpenClaw 메모]

- 이 저장소는 외부 iwinv 콘솔 DOM에 강하게 의존한다
- 문제 원인은 비즈니스 로직보다 XPath 변경일 가능성이 더 높다
- 프롬프트 제거, 삭제 확인 생략, 생성 확인 생략은 기본적으로 하지 않는 편이 안전하다
- `.env`, `state.json`, `.gocache/`, `bin/`은 로컬 런타임 산출물이다
- 문서 기준 경로는 `/root/iwinv`, 실행 바이너리는 `./bin/iwinvctl`이다
