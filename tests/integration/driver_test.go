//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sky1core/regobrick"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// =============================================================================
// 드라이버 무결성 검증 테스트 (Driver Integrity Test)
// =============================================================================
//
// 목적: DECIMAL 컬럼의 정밀도가 DB ↔ Go 전체 경로에서 손실되지 않는지 검증
//
// 검증 항목:
//   1. 드라이버 타입: DECIMAL을 float64가 아닌 string/[]byte로 반환하는지
//   2. json.Number 읽기: json.Number로 직접 스캔 시 정밀도 보존
//   3. json.Number 쓰기: json.Number를 db.Exec에 전달 시 정밀도 보존 (암묵적 string 변환)
//   4. regobrick.Number: Number.Scan/Value 경유 시 정밀도 보존
//   5. AllTypes: DECIMAL + 정수 타입(BIGINT, INTEGER) 복합 스캔
//
// 테스트 실행:
//   cd tests/integration && go test -tags=integration -v -run TestDriver
//
// =============================================================================

const sentinelDecimal = "123456789012345678901234567890123456.78"
const testInteger = int64(9223372036854775807)

// float64 정밀도 테스트용 값
const testFloatInput = "123.45678901234567890"  // 입력값 (정밀도 초과)
const testFloatExpected = "123.45678901234568" // 기대값 (float64 변환 결과)

// =============================================================================
// DB 설정 헬퍼
// =============================================================================

type dbSetup struct {
	driverName string
	connStr    string
	cleanup    func()
}

func setupPostgres(t *testing.T, ctx context.Context, driverName string) *dbSetup {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")
	connStr := fmt.Sprintf("postgres://postgres:testpass@%s:%s/testdb?sslmode=disable", host, port.Port())

	return &dbSetup{
		driverName: driverName,
		connStr:    connStr,
		cleanup: func() {
			container.Terminate(ctx)
		},
	}
}

func setupMySQL(t *testing.T, ctx context.Context) *dbSetup {
	req := testcontainers.ContainerRequest{
		Image:        "mysql:8",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "testpass",
			"MYSQL_DATABASE":      "testdb",
		},
		WaitingFor: wait.ForLog("ready for connections").
			WithOccurrence(2).
			WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "3306")
	connStr := fmt.Sprintf("root:testpass@tcp(%s:%s)/testdb", host, port.Port())

	return &dbSetup{
		driverName: "mysql",
		connStr:    connStr,
		cleanup: func() {
			container.Terminate(ctx)
		},
	}
}

func setupSQLite(t *testing.T) *dbSetup {
	return &dbSetup{
		driverName: "sqlite3",
		connStr:    ":memory:",
		cleanup:    func() {},
	}
}

func openDB(t *testing.T, setup *dbSetup) *sql.DB {
	db, err := sql.Open(setup.driverName, setup.connStr)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// MySQL 시작 대기
	if setup.driverName == "mysql" {
		for i := 0; i < 30; i++ {
			if err := db.Ping(); err == nil {
				break
			}
			time.Sleep(time.Second)
		}
	}

	return db
}

// =============================================================================
// 검증 함수
// =============================================================================

// verifyDriverType DECIMAL 컬럼에서 드라이버가 반환하는 Go 타입 검증.
// string/[]byte면 정밀도 보존, float64면 정밀도 손실.
func verifyDriverType(t *testing.T, db *sql.DB, query string) {
	var val interface{}
	if err := db.QueryRow(query).Scan(&val); err != nil {
		t.Fatalf("failed to scan: %v", err)
	}

	switch v := val.(type) {
	case float64:
		t.Fatalf("FAIL: Driver returned float64! Precision lost. Got: %v", v)
	case string:
		if v != sentinelDecimal {
			t.Fatalf("Value mismatch: got %s, want %s", v, sentinelDecimal)
		}
		t.Logf("PASS: Driver returned string")
	case []byte:
		if string(v) != sentinelDecimal {
			t.Fatalf("Value mismatch: got %s, want %s", string(v), sentinelDecimal)
		}
		t.Logf("PASS: Driver returned []byte")
	default:
		t.Fatalf("FAIL: Driver returned unexpected type %T (only string/[]byte allowed for DECIMAL)", v)
	}
}

// Stringer String() 메서드를 가진 타입
type Stringer interface {
	String() string
}

// verifyScan 제네릭 스캔 검증. T는 json.Number 또는 regobrick.Number
func verifyScan[T Stringer](t *testing.T, db *sql.DB, query, want, typeName string) {
	var v T
	if err := db.QueryRow(query).Scan(&v); err != nil {
		t.Fatalf("failed to scan into %s: %v", typeName, err)
	}

	if v.String() != want {
		t.Fatalf("%s scan wrong: got %s, want %s", typeName, v.String(), want)
	}
	t.Logf("PASS: %s scan: %s", typeName, v.String())
}

// verifyNumberValue regobrick.Number를 db.Exec에 직접 전달하여 쓰기 검증 (driver.Valuer 경로)
// createTableSQL: 테이블 생성 SQL (예: "CREATE TABLE test_value (val DECIMAL(50, 2))")
// placeholder: 파라미터 플레이스홀더 (PostgreSQL: "$1", MySQL/SQLite: "?")
func verifyNumberValue(t *testing.T, db *sql.DB, createTableSQL, placeholder string) {
	_, err := db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// regobrick.Number를 직접 전달 → driver.Valuer.Value() 호출
	n := regobrick.Number(sentinelDecimal)
	_, err = db.Exec("INSERT INTO test_value (val) VALUES ("+placeholder+")", n)
	if err != nil {
		t.Fatalf("failed to insert via Number: %v", err)
	}

	// raw string으로 읽어서 확인
	var raw string
	if err := db.QueryRow("SELECT val FROM test_value").Scan(&raw); err != nil {
		t.Fatalf("failed to scan raw: %v", err)
	}

	if raw != sentinelDecimal {
		t.Fatalf("Number.Value precision lost: got %s, want %s", raw, sentinelDecimal)
	}
	t.Logf("PASS: Number.Value preserved precision")
}

// verifyJsonNumberValue json.Number를 db.Exec에 직접 전달하여 쓰기 검증 (암묵적 string 변환 경로)
// json.Number는 type Number string이므로 driver.Valuer가 아닌
// DefaultParameterConverter.ConvertValue의 reflect.String 케이스로 처리됨.
func verifyJsonNumberValue(t *testing.T, db *sql.DB, createTableSQL, placeholder string) {
	_, err := db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// json.Number를 직접 전달 → DefaultParameterConverter가 string으로 변환
	n := json.Number(sentinelDecimal)
	_, err = db.Exec("INSERT INTO test_jn_value (val) VALUES ("+placeholder+")", n)
	if err != nil {
		t.Fatalf("failed to insert via json.Number: %v", err)
	}

	// raw string으로 읽어서 확인
	var raw string
	if err := db.QueryRow("SELECT val FROM test_jn_value").Scan(&raw); err != nil {
		t.Fatalf("failed to scan raw: %v", err)
	}

	if raw != sentinelDecimal {
		t.Fatalf("json.Number implicit string conversion precision lost: got %s, want %s", raw, sentinelDecimal)
	}
	t.Logf("PASS: json.Number implicit string conversion preserved precision")
}

// =============================================================================
// PostgreSQL pgx 테스트
// =============================================================================

func TestDriverIntegrity_Postgres_pgx(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	setup := setupPostgres(t, ctx, "pgx")
	defer setup.cleanup()

	db := openDB(t, setup)
	defer db.Close()

	_, err := db.Exec("CREATE TABLE test_decimal (val DECIMAL(50, 2))")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec("INSERT INTO test_decimal (val) VALUES ($1)", sentinelDecimal)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	t.Run("DriverType", func(t *testing.T) {
		verifyDriverType(t, db, "SELECT val FROM test_decimal")
	})

	t.Run("JsonNumber", func(t *testing.T) {
		verifyScan[json.Number](t, db, "SELECT val FROM test_decimal", sentinelDecimal, "json.Number")
	})

	t.Run("Number", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT val FROM test_decimal", sentinelDecimal, "regobrick.Number")
	})

	t.Run("NumberValue", func(t *testing.T) {
		verifyNumberValue(t, db, "CREATE TABLE test_value (val DECIMAL(50, 2))", "$1")
	})

	t.Run("JsonNumberValue", func(t *testing.T) {
		verifyJsonNumberValue(t, db, "CREATE TABLE test_jn_value (val DECIMAL(50, 2))", "$1")
	})
}

func TestDriverIntegrity_Postgres_pgx_AllTypes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	setup := setupPostgres(t, ctx, "pgx")
	defer setup.cleanup()

	db := openDB(t, setup)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE all_types (
		dec_val DECIMAL(50, 2),
		bigint_val BIGINT
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec("INSERT INTO all_types VALUES ($1, $2)",
		sentinelDecimal, testInteger)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// 드라이버 타입 검증
	t.Run("DecimalDriverType", func(t *testing.T) {
		verifyDriverType(t, db, "SELECT dec_val FROM all_types")
	})

	// regobrick.Number로 모든 타입 스캔 (json.Number는 string만 가능하므로 제외)
	t.Run("Number/DECIMAL", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT dec_val FROM all_types", sentinelDecimal, "regobrick.Number")
	})
	t.Run("Number/BIGINT", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT bigint_val FROM all_types", "9223372036854775807", "regobrick.Number")
	})
}

// =============================================================================
// PostgreSQL lib/pq 테스트
// =============================================================================

func TestDriverIntegrity_Postgres_pq(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	setup := setupPostgres(t, ctx, "postgres") // lib/pq uses "postgres"
	defer setup.cleanup()

	db := openDB(t, setup)
	defer db.Close()

	_, err := db.Exec("CREATE TABLE test_decimal (val DECIMAL(50, 2))")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec("INSERT INTO test_decimal (val) VALUES ($1)", sentinelDecimal)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	t.Run("DriverType", func(t *testing.T) {
		verifyDriverType(t, db, "SELECT val FROM test_decimal")
	})

	t.Run("JsonNumber", func(t *testing.T) {
		verifyScan[json.Number](t, db, "SELECT val FROM test_decimal", sentinelDecimal, "json.Number")
	})

	t.Run("Number", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT val FROM test_decimal", sentinelDecimal, "regobrick.Number")
	})

	t.Run("NumberValue", func(t *testing.T) {
		verifyNumberValue(t, db, "CREATE TABLE test_value (val DECIMAL(50, 2))", "$1")
	})

	t.Run("JsonNumberValue", func(t *testing.T) {
		verifyJsonNumberValue(t, db, "CREATE TABLE test_jn_value (val DECIMAL(50, 2))", "$1")
	})
}

// =============================================================================
// MySQL 테스트
// =============================================================================

func TestDriverIntegrity_MySQL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	setup := setupMySQL(t, ctx)
	defer setup.cleanup()

	db := openDB(t, setup)
	defer db.Close()

	_, err := db.Exec("CREATE TABLE test_decimal (val DECIMAL(50, 2))")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec("INSERT INTO test_decimal (val) VALUES (?)", sentinelDecimal)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	t.Run("DriverType", func(t *testing.T) {
		verifyDriverType(t, db, "SELECT val FROM test_decimal")
	})

	t.Run("JsonNumber", func(t *testing.T) {
		verifyScan[json.Number](t, db, "SELECT val FROM test_decimal", sentinelDecimal, "json.Number")
	})

	t.Run("Number", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT val FROM test_decimal", sentinelDecimal, "regobrick.Number")
	})

	t.Run("NumberValue", func(t *testing.T) {
		verifyNumberValue(t, db, "CREATE TABLE test_value (val DECIMAL(50, 2))", "?")
	})

	t.Run("JsonNumberValue", func(t *testing.T) {
		verifyJsonNumberValue(t, db, "CREATE TABLE test_jn_value (val DECIMAL(50, 2))", "?")
	})
}

func TestDriverIntegrity_MySQL_AllTypes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	setup := setupMySQL(t, ctx)
	defer setup.cleanup()

	db := openDB(t, setup)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE all_types (
		dec_val DECIMAL(50, 2),
		bigint_val BIGINT
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec("INSERT INTO all_types VALUES (?, ?)",
		sentinelDecimal, testInteger)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// 드라이버 타입 검증
	t.Run("DecimalDriverType", func(t *testing.T) {
		verifyDriverType(t, db, "SELECT dec_val FROM all_types")
	})

	// regobrick.Number로 모든 타입 스캔 (json.Number는 string만 가능하므로 제외)
	t.Run("Number/DECIMAL", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT dec_val FROM all_types", sentinelDecimal, "regobrick.Number")
	})
	t.Run("Number/BIGINT", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT bigint_val FROM all_types", "9223372036854775807", "regobrick.Number")
	})
}

// =============================================================================
// SQLite 테스트
// =============================================================================

func TestDriverIntegrity_SQLite(t *testing.T) {
	setup := setupSQLite(t)
	defer setup.cleanup()

	db := openDB(t, setup)
	defer db.Close()

	// SQLite는 DECIMAL 타입이 없으므로 TEXT 사용
	_, err := db.Exec("CREATE TABLE test_decimal (val TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec("INSERT INTO test_decimal (val) VALUES (?)", sentinelDecimal)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	t.Run("DriverType", func(t *testing.T) {
		verifyDriverType(t, db, "SELECT val FROM test_decimal")
	})

	t.Run("JsonNumber", func(t *testing.T) {
		verifyScan[json.Number](t, db, "SELECT val FROM test_decimal", sentinelDecimal, "json.Number")
	})

	t.Run("Number", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT val FROM test_decimal", sentinelDecimal, "regobrick.Number")
	})

	t.Run("NumberValue", func(t *testing.T) {
		verifyNumberValue(t, db, "CREATE TABLE test_value (val TEXT)", "?")
	})

	t.Run("JsonNumberValue", func(t *testing.T) {
		verifyJsonNumberValue(t, db, "CREATE TABLE test_jn_value (val TEXT)", "?")
	})
}

func TestDriverIntegrity_SQLite_AllTypes(t *testing.T) {
	setup := setupSQLite(t)
	defer setup.cleanup()

	db := openDB(t, setup)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE all_types (
		text_val TEXT,
		int_val INTEGER,
		real_val REAL,
		real_precision_val REAL
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec("INSERT INTO all_types VALUES (?, ?, ?, ?)",
		sentinelDecimal, testInteger, 123.456, testFloatInput)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// 드라이버 타입 검증
	t.Run("TextDriverType", func(t *testing.T) {
		verifyDriverType(t, db, "SELECT text_val FROM all_types")
	})

	// regobrick.Number로 모든 타입 스캔 (json.Number는 string만 가능하므로 제외)
	t.Run("Number/TEXT", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT text_val FROM all_types", sentinelDecimal, "regobrick.Number")
	})
	t.Run("Number/INTEGER", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT int_val FROM all_types", "9223372036854775807", "regobrick.Number")
	})
	t.Run("Number/REAL", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT real_val FROM all_types", "123.456", "regobrick.Number")
	})
	t.Run("Number/REAL_PrecisionLoss", func(t *testing.T) {
		// 정밀도 초과 값이 float64 변환 결과와 일치하는지 검증
		verifyScan[regobrick.Number](t, db, "SELECT real_precision_val FROM all_types", testFloatExpected, "regobrick.Number")
	})
}
