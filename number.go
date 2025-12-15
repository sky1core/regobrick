package regobrick

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
)

// Number Rego에 전달하는 숫자 표현.
// JSON 직렬화 시 숫자 리터럴로 출력되고, DB의 숫자 컬럼과 직접 연동된다.
//
// 계약:
//   - 지수표기(e/E)는 미지원. 지수표기가 들어오면 UseDecimalArithmetic()의
//     연산에서 udecimal 파싱 에러로 평가가 실패한다.
//   - 입력값의 유효성은 제공자 책임. regobrick은 연산 시점에만 검증한다.
//   - DB Scan 시 string, []byte, int64, float64 모두 허용.
//   - DECIMAL 컬럼의 정밀도 보장은 드라이버 책임 (string/[]byte 반환 필요).
//     드라이버 무결성 테스트로 사전 검증 권장.
//
// NULL/빈값 처리 (json.Number 및 decimal 라이브러리 관례):
//   - JSON: null → 빈 문자열, 빈 문자열 → 0으로 출력 (Go zero value 관례)
//   - DB: NULL/빈값 → 에러 (decimal 라이브러리 관례)
//
// 예시:
//
//	input := map[string]any{
//	    "price": regobrick.Number("123.45"),
//	}
type Number json.Number

// String 문자열 표현 반환.
func (n Number) String() string {
	return string(n)
}

// MarshalJSON JSON 숫자 리터럴로 출력 (따옴표 없이).
// json.Number는 빈 문자열("")을 0으로 출력한다 (Go 관례: zero value는 유효).
func (n Number) MarshalJSON() ([]byte, error) {
	return json.Marshal(json.Number(n))
}

// UnmarshalJSON JSON 숫자를 파싱.
// json.Number는 null을 빈 문자열("")로 처리한다.
// 빈 문자열은 MarshalJSON에서 0으로 출력된다.
func (n *Number) UnmarshalJSON(b []byte) error {
	var num json.Number
	if err := json.Unmarshal(b, &num); err != nil {
		return err
	}
	*n = Number(num)
	return nil
}

// Scan sql.Scanner 구현. DB에서 숫자 컬럼 읽기.
// NULL/빈값은 에러 반환 (udecimal, shopspring 등 decimal 라이브러리 관례).
func (n *Number) Scan(src any) error {
	if src == nil {
		return fmt.Errorf("Number.Scan: NULL not supported")
	}
	switch v := src.(type) {
	case string:
		if v == "" {
			return fmt.Errorf("Number.Scan: empty string not supported")
		}
		*n = Number(v)
	case []byte:
		if len(v) == 0 {
			return fmt.Errorf("Number.Scan: empty value not supported")
		}
		*n = Number(v)
	case int64:
		*n = Number(strconv.FormatInt(v, 10))
	case float64:
		*n = Number(strconv.FormatFloat(v, 'f', -1, 64))
	default:
		return fmt.Errorf("Number.Scan: unsupported type %T", src)
	}
	return nil
}

// Value driver.Valuer 구현. DB에 DECIMAL/NUMERIC 컬럼 쓰기.
// 빈값은 에러 반환 (decimal 라이브러리 관례).
func (n Number) Value() (driver.Value, error) {
	if n == "" {
		return nil, fmt.Errorf("Number.Value: empty value not supported")
	}
	return string(n), nil
}
