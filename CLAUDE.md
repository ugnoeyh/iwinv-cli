# CLAUDE.md

## 프로젝트 개요
- 이 프로젝트는 iwinv 콘솔을 Playwright로 자동화하는 Go CLI다.
- 구조는 `cmd/<binary>` + `internal/*` + `docs/` 패턴을 따른다.
- 외부 진입점은 얇게 유지하고, 실제 기능 로직은 `internal` 패키지에 둔다.

## 현재 디렉토리 구조
- `cmd/iwinvctl`
  실행 진입점
- `internal/cli`
  플래그 정의, 실행 흐름, 기능 라우팅
- `internal/runtime`
  Playwright 초기화, 로그인, 세션 저장/복원
- `internal/console`
  iwinv 콘솔 자동화 로직 전반
- `internal/domain`
  공용 타입
- `internal/envfile`
  `.env` 로더
- `internal/ui`
  사용자 입력/확인 프롬프트
- `internal/output`
  출력 포맷터
- `docs`
  구조 문서

## 유지보수 원칙
- 새 명령 플래그나 분기는 먼저 `internal/cli`에서 추가한다.
- 브라우저/세션 처리 변경은 `internal/runtime` 안에서만 다룬다.
- 콘솔 DOM 구조 변경은 먼저 `internal/console/constants.go`를 점검한다.
- 선택/조회 공통 로직은 `internal/console/options.go`에 모은다.
- 생성 요약 출력 규칙은 `internal/output/summary.go`에서 관리한다.

## 수정 시 우선 확인할 곳
1. 로그인 실패 또는 세션 문제: `internal/runtime/runtime.go`
2. XPath 변경: `internal/console/constants.go`
3. 옵션 선택 실패: `internal/console/options.go`
4. 서버 목록/전원/IP/삭제 문제: `internal/console/servers.go`
5. 리전/스펙/OS 조회 문제: `internal/console/query.go`
6. 생성 플로우 문제: `internal/console/create.go`

## 권장 작업 흐름
1. 명령 추가 여부를 먼저 `internal/cli/run.go`에서 판단한다.
2. 공용 타입이 필요하면 `internal/domain`에 둔다.
3. 재사용 가능한 출력 포맷은 `internal/output`으로 분리한다.
4. 기능 수정 후에는 최소한 `gofmt`와 가능한 범위의 빌드 검증을 수행한다.

## 실행 방법
```powershell
go run ./cmd/iwinvctl --list
go run ./cmd/iwinvctl --create-region1
go run ./cmd/iwinvctl --power off --target my-server
go run ./cmd/iwinvctl --create --region1 "KR" --region2 "SEOUL" --spec "2 vCore" --name "auto-server"
```

## 주의사항
- `state.json`과 `.env`는 민감 정보가 될 수 있으므로 커밋하지 않는 편이 안전하다.
- 삭제와 생성은 실제 리소스에 영향을 주므로 확인 프롬프트를 유지한다.
- 외부 콘솔 DOM에 강하게 의존하므로, UI 변경 시 로직보다 XPath/선택 규칙부터 확인한다.
