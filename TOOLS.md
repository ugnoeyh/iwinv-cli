# TOOLS.md

## iwinvctl (Linux, MAC,  OpenClaw 전용 실행 가이드)

# IWINV-CLI

## 1) 고정 작업 경로
- 작업 디렉터리: `/usr/local/bin`
- 실행 바이너리: `/usr/local/bin/iwinvctl`
- 환경파일: `/usr/local/bin/.env`
- 세션파일: `/usr/local/bin/state.json` (현재 작업 디렉터리 기준)

## 2) 기본 실행 템플릿
모든 명령은 아래 템플릿을 기준으로 실행합니다.

```bash
/usr/local/bin/iwinvctl <flags...>
```

## 3) 로그인/세션 규칙
- `state.json`이 있으면 저장 세션을 재사용합니다.
- 세션이 만료되었거나 꼬였으면 아래로 재로그인합니다.

주의:
- 최초 로그인 시 ID/PW 입력이 필요합니다.

## 5) 보안 규칙
- `.env`에 `IWINV_PW` 저장 후 사용


예시 `.env`:
```env
IWINV_ID=your-id
IWINV_PW=your-password
```

## 6) 액션별 명령어

서버 조회:
```bash
/usr/local/bin/iwinvctl`
```

서버 트래픽 조회(일일 상세):
```bash
/usr/local/bin/iwinvctl --traffic --target "<서버명 또는 server-idx>"
/usr/local/bin/iwinvctl --traffic --target "<서버명 또는 server-idx>" --traffic-year <Year> --traffic-month <Month>
```

요금 상세 조회(서버/상품별):
```bash
/usr/local/bin/iwinvctl --bill
/usr/local/bin/iwinvctl --bill --bill-year 2026 --bill-month 4
/usr/local/bin/iwinvctl --bill --target "<서버IP 또는 server-idx>"
```

전원 제어:
```bash
/usr/local/bin/iwinvctl --power on --target "<서버명 또는 server-idx>"
/usr/local/bin/iwinvctl --power off --target "<서버명 또는 server-idx>"
```

공인 IP 제어:
```bash
/usr/local/bin/iwinvctl --ip on --target "<서버명 또는 server-idx>"
/usr/local/bin/iwinvctl --ip off --target "<서버명 또는 server-idx>"
```

서버 삭제:
```bash
/usr/local/bin/iwinvctl --delete --target "<서버명 또는 IDX>"
```

예시 생성용 메타 조회:
```bash
/usr/local/bin/iwinvctl --create-region1
/usr/local/bin/iwinvctl --create-region2 "KR1-ZONE"
/usr/local/bin/iwinvctl --create-spec --region1 "KR1-ZONE" --region2 "KR1-Z02"
/usr/local/bin/iwinvctl --create-os --region1 "KR1-ZONE" --region2 "KR1-Z02" --spec "hpa_2.4-g2"
/usr/local/bin/iwinvctl --search-spec "2 vCore"
```

예시 서버 생성:
```bash
/usr/local/bin/iwinvctl --create \
  --region1 "KR1-ZONE" \
  --region2 "KR1-Z02" \
  --spec "hpa_2.4-g2" \
  --os-type "기본" \
  --os "Ubuntu 22.04" \
  --name "auto-server"
```

방화벽 목록/탭 조회:
```bash
/usr/local/bin/iwinvctl --firewall-list
/usr/local/bin/iwinvctl --firewall-create --firewall "<firewalld-name>"
/usr/local/bin/iwinvctl --firewall-tab inbound --firewall-ref "<firewalld-idx>"
/usr/local/bin/iwinvctl --firewall-tab outbound --firewall-ref "<firewalld-idx>"
/usr/local/bin/iwinvctl --firewall-tab international --firewall-ref "<firewalld-idx>"
/usr/local/bin/iwinvctl --firewall-tab bot --firewall-ref "<firewalld-idx>"
```

방화벽 룰 추가/삭제:
```bash
/usr/local/bin/iwinvctl --firewall-add --firewall-ref "<firewalld-idx>" --firewall-dir inbound --rule-protocol TCP --rule-port <PORT> --rule-ip "<IP>"
/usr/local/bin/iwinvctl --firewall-remove --firewall-ref "<firewalld-idx>" --firewall-dir inbound --rule-protocol TCP --rule-port <PORT> --rule-ip "<IP>"
```

국제망/검색봇 정책:
```bash
/usr/local/bin/iwinvctl --firewall-ref "<firewalld-idx>" --firewall-international "FOREIGN"
/usr/local/bin/iwinvctl --firewall-ref "<firewalld-idx>" --firewall-international-remove "CHINA,USA"
/usr/local/bin/iwinvctl --firewall-ref "<firewalld-idx>" --firewall-international-clear
/usr/local/bin/iwinvctl --firewall-ref "<firewalld-idx>" --firewall-bot-remove "NAVER"
/usr/local/bin/iwinvctl --firewall-ref "<firewalld-idx>" --firewall-bot-clear
```

서버별 방화벽 사용 설정:
```bash
/usr/local/bin/iwinvctl --firewall-choice-server "<server-idx>" --firewall-choice-policy ""<firewalld-idx>" --firewall-choice-use "Y"
/usr/local/bin/iwinvctl --firewall-choice-server "<server-idx>" --firewall-choice-policy ""<firewalld-idx>" --firewall-choice-use "N"
```

필수 인자/충돌 규칙:
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

