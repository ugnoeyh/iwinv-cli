# Architecture

`iwinv-cli`는 iwinv 콘솔 웹 세션을 재사용해 인프라 작업을 자동화하는 비공식 CLI입니다.
현재 구조는 `Playwright 기반 로그인/세션 관리 + Go CLI + 로컬 상태 파일 + iwinv 콘솔 웹 API/DOM 자동화`로 나뉩니다.

이 문서는 현재 저장소 구현을 기준으로 아키텍처를 정리합니다. 목표는 두 가지입니다.

- 사람이 전체 구조를 빠르게 이해할 수 있게 하기
- LLM/자동화 도구가 같은 경계를 기준으로 기능을 안전하게 확장할 수 있게 하기

## 현재 상태

현재 구현 범위는 아래와 같습니다.

- 브라우저 로그인 기반 세션 저장/재사용 (`state.json`)
- 서버 목록 조회
- 서버 전원 제어 (`on/off`)
- 서버 공인 IP 제어 (`on/off`)
- 서버 영구 삭제 (확인 프롬프트 포함)
- 서버 생성 관련 조회
  - 리전 1차/2차 조회
  - 스펙 조회
  - OS 목록 조회
  - 특정 스펙 가능 리전 역추적 검색
- 서버 생성 자동화
  - 리전/스펙/OS 선택
  - 수량/이름 설정
  - 블록 스토리지 선택
  - 방화벽 선택
  - 최종 요약 후 확인 생성
- ELCAP 방화벽 자동화
  - 정책 목록 조회
  - 탭 조회 (`inbound`, `outbound`, `international`, `bot`)
  - 룰 추가/삭제 (`inbound`, `outbound`, `both`)
  - 국제망 정책 설정/개별 제거/전체 제거
  - 검색봇 정책 설정/개별 제거/전체 제거
  - 서버 단위 방화벽 사용 설정 (`firewall/choice`, `use=Y|N`)

## 설계 원칙

- 기본은 조회 중심이며 mutation은 명시적 플래그로만 실행
- 로그인/세션 관리와 도메인 자동화 로직을 분리
- CLI 진입점은 얇게 유지하고 핵심 동작은 `internal/*`로 격리
- 고위험 작업은 추가 확인 또는 사전 검증을 거친 뒤 실행
- 콘솔 DOM 변경에 대비해 XPath/선택 로직을 공통화
- 저장 후 재조회 검증(특히 방화벽 변경)을 통해 성공 오탐 최소화

## System Context

```mermaid
C4Context
title 시스템 컨텍스트 - iwinv-cli

Person(user, "User", "터미널에서 조회/제어/생성/삭제를 실행")
Person(agent, "Agent / Automation", "Codex, Claude, shell script 등 상위 자동화 계층")

System(cli, "iwinvctl", "iwinv 콘솔 자동화 CLI")

System_Ext(iwinvWeb, "iwinv Console Web", "로그인/콘솔 UI")
System_Ext(iwinvApi, "iwinv Console APIs", "조회/제어/저장 요청 엔드포인트")
System_Ext(playwright, "Playwright Chromium", "브라우저 세션 확보 및 페이지 자동화")

Rel(user, cli, "실행", "CLI")
Rel(agent, cli, "호출", "CLI / shell")
Rel(cli, playwright, "런타임에서 실행", "playwright-go")
Rel(playwright, iwinvWeb, "브라우저 로그인/DOM 조작", "HTTPS")
Rel(playwright, iwinvApi, "조회/제어/저장", "HTTPS + session cookies")
```

## Container View

```mermaid
C4Container
title 컨테이너 다이어그램 - iwinv-cli

Person(user, "User", "직접 사용하는 운영자")
Person(agent, "Agent", "LLM 또는 자동화 스크립트")

System_Boundary(system, "iwinv-cli") {
  Container(goCli, "iwinvctl", "Go", "메인 CLI 바이너리")
  Container(runtime, "runtime", "playwright-go", "브라우저 실행, 로그인, 세션 검증/복원")
  ContainerDb(localFiles, "Local State Files", "JSON / ENV", "state.json, .env")
  Container(console, "console automation", "Go + JS evaluate", "서버/생성/방화벽 자동화 로직")
  Container(docs, "Docs", "Markdown", "README, TOOLS, ARCHITECTURE")
}

System_Ext(iwinvWeb, "iwinv Console Web", "로그인 화면/콘솔 페이지")
System_Ext(iwinvApi, "iwinv Console APIs", "서버/방화벽 관련 엔드포인트")

Rel(user, goCli, "Uses")
Rel(agent, goCli, "Calls", "shell")
Rel(goCli, runtime, "초기화/페이지 생성/세션 저장")
Rel(runtime, iwinvWeb, "로그인/세션 유효성 확인")
Rel(goCli, console, "명령별 실행 위임")
Rel(console, iwinvWeb, "페이지 진입/DOM 파싱", "via Playwright")
Rel(console, iwinvApi, "조회/수정 요청", "via Playwright fetch")
Rel(runtime, localFiles, "Read/Write state.json")
Rel(goCli, localFiles, "Read .env")
Rel(goCli, docs, "개발/운영 가이드 참조")
```

## Component View

```mermaid
C4Component
title 컴포넌트 다이어그램 - iwinvctl

Container(cli, "iwinvctl", "Go CLI", "Main binary")

Container_Boundary(core, "Application Core") {
  Component(entry, "cmd/iwinvctl", "main", "CLI 진입점")
  Component(flags, "internal/cli/flags", "flag parser", "플래그/환경변수 기본값 로딩")
  Component(router, "internal/cli/run", "action router", "명령 분기와 실행 흐름")
  Component(rt, "internal/runtime", "runtime", "Playwright 실행, 로그인, 세션 관리")
  Component(serverSvc, "internal/console/servers", "server automation", "목록/전원/IP/삭제")
  Component(querySvc, "internal/console/query", "query automation", "리전/스펙/OS 조회, 스펙 역추적")
  Component(createSvc, "internal/console/create", "create automation", "생성 플로우 자동화")
  Component(fwSvc, "internal/console/firewall", "firewall automation", "정책/룰/국제망/봇")
  Component(fwChoiceSvc, "internal/console (firewall_choice.go)", "server firewall choice", "서버별 방화벽 사용 설정")
  Component(waitSvc, "internal/console/wait", "wait helpers", "로딩 대기/가시성 체크")
  Component(optionSvc, "internal/console/options", "option helpers", "선택/파싱 공통화")
  Component(uiSvc, "internal/ui", "prompt helpers", "로그인/삭제/생성 확인 입력")
  Component(outSvc, "internal/output", "formatter", "생성 요약 출력 포맷")
}

Rel(entry, flags, "parse")
Rel(flags, router, "Flags 전달")
Rel(router, rt, "Runtime 초기화")
Rel(router, serverSvc, "run")
Rel(router, querySvc, "run")
Rel(router, createSvc, "run")
Rel(router, fwSvc, "run")
Rel(router, fwChoiceSvc, "run")
Rel(createSvc, optionSvc, "선택/파싱")
Rel(createSvc, waitSvc, "대기/조건 체크")
Rel(serverSvc, uiSvc, "삭제 확인")
Rel(createSvc, uiSvc, "생성 확인")
Rel(createSvc, outSvc, "요약 렌더링")
```

## Key Flows

### 1. Browser-assisted login and session reuse

```mermaid
sequenceDiagram
    actor User
    participant CLI as iwinvctl
    participant Runtime as internal/runtime
    participant Browser as Playwright Chromium
    participant Web as iwinv Console Web
    participant State as state.json

    User->>CLI: --login 또는 일반 명령 실행
    CLI->>Runtime: New(resetLogin?)
    Runtime->>State: 기존 state.json 확인
    alt state 존재 + 유효
        Runtime->>Browser: StorageStatePath 로 컨텍스트 생성
        Runtime-->>CLI: 재사용 컨텍스트 반환
    else state 없음 또는 만료
        User->>CLI: ID / PW 입력
        Runtime->>Browser: 신규 컨텍스트/페이지 생성
        Browser->>Web: 로그인 페이지 접속
        CLI->>Browser: ID/PW 입력 후 제출
        Runtime->>State: 세션 저장
        Runtime-->>CLI: 신규 컨텍스트 반환
    end
```

### 2. Read-only flow (list/query/firewall-tab)

```mermaid
sequenceDiagram
    actor Caller as User / Agent
    participant CLI as internal/cli
    participant Runtime as internal/runtime
    participant Console as internal/console/*
    participant Web as iwinv Console

    Caller->>CLI: --list / --create-spec / --firewall-tab ...
    CLI->>Runtime: 페이지 생성
    CLI->>Console: Run* 호출
    Console->>Web: 페이지 이동 + DOM 파싱
    Web-->>Console: HTML/JSON 응답
    Console-->>Caller: 표 형태 결과 출력
```

### 3. Firewall mutation flow (add/remove/international/bot/choice)

```mermaid
sequenceDiagram
    actor Caller as User / Agent
    participant CLI as internal/cli/run
    participant FW as internal/console/firewall*
    participant Web as iwinv Console APIs

    Caller->>CLI: --firewall-* mutation flags
    CLI->>FW: 입력 정규화 + 대상 정책/서버 해석
    FW->>Web: 현재 폼/탭 스냅샷 조회
    FW->>Web: POST 저장 요청
    Web-->>FW: 저장 응답
    FW->>Web: 즉시 재조회(검증)
    alt 검증 성공
      FW-->>Caller: 성공 출력
    else 검증 실패
      FW-->>Caller: 에러 반환(성공 오탐 방지)
    end
```

### 4. Server creation flow

```mermaid
sequenceDiagram
    actor User
    participant CLI as iwinvctl --create
    participant CreateSvc as internal/console/create
    participant Web as iwinv Console

    User->>CLI: --create ...
    CLI->>CreateSvc: RunCreate
    CreateSvc->>Web: 리전/스펙/OS/옵션 선택
    CreateSvc->>Web: 확인 페이지 이동
    Web-->>CreateSvc: 요약 정보
    CreateSvc-->>User: 요약 출력 + 확인 프롬프트
    User->>CreateSvc: y / n
    CreateSvc->>Web: 최종 생성 요청
    Web-->>CreateSvc: success URL
    CreateSvc-->>User: 생성 완료
```

## Safety Model

mutation(생성/삭제/방화벽 수정)은 아래 순서로 안전 장치를 적용합니다.

1. 세션 게이트
   - 페이지 이동 후 로그인 페이지 여부 검사
   - 세션 만료 시 즉시 중단 + 재로그인 안내
2. 필수 파라미터 게이트
   - 예: `--firewall-ref`, `--rule-ip`, `--rule-port`, `--target` 누락 시 실행 중단
3. 사전 상태 확인
   - 정책/서버 식별자 resolve
   - 중복 룰 검사 또는 삭제 대상 존재 검사
4. 저장 응답 검사
   - HTTP/본문/키워드/JSON 코드 확인
5. 저장 후 재조회 검증
   - 룰 추가/삭제, 국제망/봇, 서버 방화벽 choice는 재조회 결과가 기대와 일치해야 성공 처리
6. 사용자 확인 프롬프트
   - 서버 삭제: 반드시 확인
   - 서버 생성: 최종 요약 확인 후 생성

## Local State

로컬 상태는 아래 파일로 관리됩니다.

| File | Role |
|---|---|
| `state.json` | 브라우저 로그인 세션 저장/재사용 |
| `.env` | 기본 비밀번호 등 환경변수 (`IWINV_PW`) |

기본적으로 `state.json`은 실행 디렉터리(보통 바이너리 실행 위치)에 생성됩니다.

## Package Map

| Package | Role |
|---|---|
| `cmd/iwinvctl` | 명령 진입점 |
| `internal/cli` | 플래그 파싱, 액션 라우팅 |
| `internal/runtime` | Playwright 실행, 로그인, 세션 저장/복원 |
| `internal/console` | iwinv 콘솔 자동화 로직 전반 |
| `internal/console` (`firewall_choice.go` 포함) | iwinv 콘솔 자동화 로직 전반 (서버별 방화벽 사용 설정 포함) |
| `internal/domain` | 공용 타입 |
| `internal/envfile` | `.env` 로더 |
| `internal/ui` | 프롬프트/확인 입력 |
| `internal/output` | 생성 요약 포맷터 |

## Current Gaps

현재 남아 있는 주요 항목입니다.

- 외부 콘솔 DOM/XPath 변경에 취약 (회귀 테스트 자동화 필요)
- 네트워크/API 실패 시 재시도 전략 표준화 미흡
- 일부 mutation에 대해 통합 테스트(실서버 모의 계정 기반) 부족
- 방화벽 관련 기능은 고도화되었지만 API 스키마 변경 시 빠른 탐지 장치 부족

## Related Docs

- [`README.md`](../README.md)
- [`TOOLS.md`](../TOOLS.md)
- [`CLAUDE.md`](../CLAUDE.md)
