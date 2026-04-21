# iwinv-cli

iwinv 콘솔을 Playwright로 자동화하는 Go CLI 프로젝트입니다.  
서버 조회, 전원/IP 제어, 리전/스펙/OS 조회, 서버 생성, 서버 삭제까지 터미널에서 처리할 수 있도록 구성되어 있습니다.

## 주요 기능
- 내 서버 목록 조회
- 서버 전원 `on/off`
- 서버 공인 IP `on/off`
- 서버 영구 삭제
- 생성 가능한 리전 1차/2차 조회
- 서버 스펙 조회
- 운영체제(OS) 목록 조회
- ELCAP 방화벽 정책 목록 조회
- ELCAP 정책 탭 조회(인바운드/아웃바운드/국제망/봇)
- ELCAP 룰 추가(인바운드/아웃바운드/양방향)
- 특정 스펙이 가능한 리전 역추적 검색
- 서버 생성 자동화

## 기술 스택
- Go
- [playwright-go](https://github.com/playwright-community/playwright-go)
- Playwright Chromium

## 요구사항
- Go 1.26 이상
- iwinv 계정

## 설치
```cmd
git clone <your-repo-url>
cd iwinv-cli
go mod tidy
```

처음 실행 시 Playwright와 브라우저 리소스가 자동 설치될 수 있습니다.

## 빠른 시작
### 1. 로그인 세션 생성
```cmd
./iwinvctl --login
```

- 처음 로그인하면 `state.json`이 생성됩니다.
- 이후에는 저장된 세션을 재사용합니다.
- 세션을 다시 만들고 싶으면 다시 `--login`을 실행하면 됩니다.

### 2. `.env` 설정 선택사항
삭제나 다중 서버 생성 시 사용할 비밀번호를 `.env`에 넣어둘 수 있습니다.

```env
IWINV_PW=your-password
```

## 사용 예시
### 서버 목록 조회
```cmd
./iwinvctl --list
```

### ELCAP 방화벽 정책 목록 조회
```cmd
./iwinvctl --firewall-list
```

### ELCAP 방화벽 정책 생성
```cmd
./iwinvctl --firewall-create --firewall "정책명"
```

### 기술지원 작성 페이지 바로 열기 (동의 자동 처리)
```cmd
./iwinvctl --support-write
```

### ELCAP 정책 탭 조회
```cmd
./iwinvctl --firewall-tab inbound --firewall-ref "[firewall-idx]"
```

```cmd
./iwinvctl --firewall-tab outbound --firewall-ref "[firewall-idx]"
```

```cmd
./iwinvctl --firewall-tab international --firewall-ref "[firewall-idx]"
```

```cmd
./iwinvctl --firewall-tab bot --firewall-ref "[firewall-idx]"
```

### ELCAP 룰 추가 (인바운드/아웃바운드)
```cmd
./iwinvctl --firewall-add --firewall-ref "[firewall-idx]" --firewall-dir inbound --rule-protocol TCP --rule-port "[port]" --rule-ip "[server ip]"
```

```cmd
./iwinvctl --firewall-add --firewall-ref "[firewall-idx]" --firewall-dir outbound --rule-protocol TCP --rule-port "[port]" --rule-ip "[server ip]"
```

### ELCAP 룰 추가 (인바운드+아웃바운드 동시)
```cmd
./iwinvctl --firewall-add --firewall-ref "[firewall-idx]" --firewall-dir both --rule-protocol TCP --rule-port "[port]" --rule-ip "[server ip]"
```

### ELCAP 룰 추가 디버그
```cmd
./iwinvctl --firewall-add --firewall-ref "[firewall-idx]" --firewall-dir inbound --rule-protocol TCP --rule-port "[port]" --rule-ip "[server ip]" --firewall-debug
```

### ELCAP 룰 삭제 (인바운드/아웃바운드)
```cmd
./iwinvctl --firewall-remove --firewall-ref "[firewall-idx]" --firewall-dir inbound --rule-protocol TCP --rule-port "[port]" --rule-ip "[server ip]"
```

```cmd
./iwinvctl --firewall-remove --firewall-ref "[firewall-idx]" --firewall-dir outbound --rule-protocol TCP --rule-port "[port]" --rule-ip "[server ip]"
```

```cmd
./iwinvctl --firewall-remove --firewall-ref "[firewall-idx]" --firewall-dir both --rule-protocol TCP --rule-port "[port]" --rule-ip "[server ip]"
```

### ELCAP 국제망 통신 정책 설정 (5개국 선택)
```cmd
./iwinvctl --firewall-ref "[firewall-idx]" --firewall-international "TAIWAN,CHINA,PHILIPPINES,USA,JAPAN"
```

### ELCAP 국제망 통신 정책 설정 (한국 제외 전체 차단)
```cmd
./iwinvctl --firewall-ref "[firewall-idx]" --firewall-international "FOREIGN"
```

- `FOREIGN`은 단독으로만 사용 가능합니다.
- 선택 가능한 국가 코드는 `TAIWAN`, `CHINA`, `PHILIPPINES`, `USA`, `JAPAN` 입니다.



### ELCAP 국제망 통신 정책 개별 제거
```cmd
./iwinvctl --firewall-ref "[firewall-idx]" --firewall-international-remove "TAIWAN" --firewall-debug
```

```cmd
./iwinvctl --firewall-ref "[firewall-idx]" --firewall-international-remove "CHINA,USA" --firewall-debug
```

### ELCAP 국제망 통신 정책 전체 제거
```cmd
./iwinvctl --firewall-ref "[firewall-idx]" --firewall-international-clear --firewall-debug
```

### ELCAP 검색봇 차단 정책 설정
```cmd
./iwinvctl --firewall-ref "[firewall-idx]" --firewall-bot "GOOGLE,NAVER"
```

### ELCAP 검색봇 차단 정책 설정 (전체 검색봇 차단)
```cmd
./iwinvctl --firewall-ref "[firewall-idx]" --firewall-bot "ALL"
```

- `ALL`은 단독으로만 사용 가능합니다.
- 선택 가능한 검색봇 코드는 `GOOGLE`, `NAVER`, `DAUM` 입니다.

### ELCAP 검색봇 차단 정책 개별 제거
```cmd
./iwinvctl --firewall-ref "[firewall-idx]" --firewall-bot-remove "NAVER" --firewall-debug
```

```cmd
./iwinvctl --firewall-ref "[firewall-idx]" --firewall-bot-remove "GOOGLE,DAUM" --firewall-debug
```

### ELCAP 검색봇 차단 정책 전체 제거
```cmd
./iwinvctl --firewall-ref "[firewall-idx]" --firewall-bot-clear --firewall-debug
```

### 서버 방화벽 사용 설정 (firewall/choice)
```cmd
./iwinvctl --firewall-choice-server "[server-idx]" --firewall-choice-policy "[firewall-idx]" --firewall-choice-use "Y" --firewall-debug
```

```cmd
./iwinvctl --firewall-choice-server "[server-idx]" --firewall-choice-policy "[firewall-idx]" --firewall-choice-use "N" --firewall-debug
```

- `--firewall-choice-use`: `Y|N` 또는 `on|off`
- 서버/방화벽 식별자는 IDX 또는 이름(부분 매칭)을 지원

### 서버 전원 끄기
```cmd
./iwinvctl --power off --target my-server
```

### 서버 전원 켜기
```cmd
./iwinvctl --power on --target my-server
```

### 공인 IP 비활성화
```cmd
./iwinvctl --ip off --target my-server
```

### 서버 삭제
```cmd
./iwinvctl --delete --target my-server
```
./iwinvctl --firewall-ref "8761" --firewall-bot-clear
./iwinvctl --firewall-tab bot --firewall-ref "8761"

또는

```cmd
./iwinvctl --delete --target my-server --pw your-password
```

### 리전 1차 조회
```cmd
./iwinvctl --create-region1
```

### 특정 리전 1차의 리전 2차 조회
```cmd
./iwinvctl --create-region2 "KR1-Lite"
```

### 스펙 조회
```cmd
./iwinvctl --create-spec --region1 "KR1-ZONE" --region2 "KR1-Z02"
```

### 운영체제 목록 조회
```cmd
./iwinvctl --create-os --region1 "KR1-ZONE" --region2 "KR1-Z02" --spec "hpa_4.8-g1"
```

### 특정 스펙 가능한 리전 검색
```cmd
./iwinvctl --search-spec "2 vCore"
```

### 서버 생성
```cmd
./iwinvctl --create `
  --region1 "KR1-ZONE" `
  --region2 "KR1-Z02" `
  --spec "hpa_4.8-g1" `
  --os-type "기본" `
  --os "Ubuntu 22.04" `
  --name "서버이름"
```


## 주요 옵션
### 조회/검색
- `--create-region1`
- `--create-region2 "<region1>"`
- `--create-spec`
- `--create-os`
- `--firewall-list`
- `--firewall-create --firewall "<policy-name>"`
- `--firewall-tab "inbound|outbound|international|bot" --firewall-ref "<idx-or-name>"`
- `--firewall-add --firewall-ref "<idx-or-name>" --firewall-dir "inbound|outbound|both" --rule-ip "<ip/cidr>" --rule-port "<port>"`
- `--firewall-remove --firewall-ref "<idx-or-name>" --firewall-dir "inbound|outbound|both" --rule-ip "<ip/cidr>" --rule-port "<port>"`
- `--firewall-international "FOREIGN|TAIWAN,CHINA,PHILIPPINES,USA,JAPAN" --firewall-ref "<idx-or-name>"`
- `--firewall-international-remove "TAIWAN|CHINA,USA|..." --firewall-ref "<idx-or-name>"`
- `--firewall-international-clear --firewall-ref "<idx-or-name>"`
- `--firewall-bot "ALL|GOOGLE,NAVER,DAUM" --firewall-ref "<idx-or-name>"`
- `--firewall-bot-remove "GOOGLE|NAVER,DAUM|..." --firewall-ref "<idx-or-name>"`
- `--firewall-bot-clear --firewall-ref "<idx-or-name>"`
- `--firewall-choice-server "<server-idx-or-name>" --firewall-choice-policy "<firewall-idx-or-name>" --firewall-choice-use "Y|N|on|off"`
- `--firewall-debug`
- `--support-write`
- `--search-spec "<spec>"`

### 서버 관리
- `--list`
- `--power on|off --target "<name-or-idx>"`
- `--ip on|off --target "<name-or-idx>"`
- `--delete --target "<name-or-idx>"`

### 서버 생성
- `--create`
- `--region1 "<value>"`
- `--region2 "<value>"`
- `--spec "<value>"`
- `--os-type "기본|MY|솔루션"`
- `--os "<value>"`
- `--name "<server-name>"`
- `--qty <number>`


## 프로젝트 구조
```text
cmd/
  iwinvctl/         CLI 진입점
internal/
  cli/              플래그와 실행 흐름
  console/          iwinv 콘솔 자동화 로직
  domain/           공용 타입
  envfile/          .env 로더
  output/           출력 포맷터
  runtime/          Playwright 세션/로그인
  ui/               입력/확인 프롬프트
docs/
  ARCHITECTURE.md   구조 문서
```

## 개발 메모
- `state.json`은 로그인 세션 캐시 파일입니다.
- `.env`와 `state.json`은 민감 정보가 될 수 있으므로 커밋하지 않는 편이 안전합니다.
- iwinv 콘솔 DOM 구조가 바뀌면 `internal/console/constants.go`와 `internal/console/options.go`를 먼저 점검하세요.
- 유지보수 기준은 [`CLAUDE.md`](./CLAUDE.md), 구조 설명은 [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md)를 참고하세요.


## 주의사항
- 이 프로젝트는 실제 iwinv 리소스에 영향을 줍니다.
- 생성 및 삭제는 비용 또는 데이터 손실로 이어질 수 있으므로 확인 메시지를 주의해서 확인하세요.
- 외부 콘솔 UI가 변경되면 일부 자동화가 동작하지 않을 수 있습니다.
