package regobrick

import "encoding/json"

// Number Rego에 전달하는 숫자 표현.
// json.Number의 alias로, JSON 직렬화 시 숫자 리터럴로 출력된다.
//
// 계약:
//   - 지수표기(e/E)는 미지원. 지수표기가 들어오면 RegisterOperatorOverloads()의
//     연산에서 udecimal 파싱 에러로 평가가 실패한다.
//   - 입력값의 유효성은 제공자 책임. regobrick은 연산 시점에만 검증한다.
//
// 예시:
//
//	input := map[string]any{
//	    "price": regobrick.Number("123.45"),
//	}
type Number = json.Number
