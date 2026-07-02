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
// Driver Integrity Test
// =============================================================================
//
// Purpose: Verify that DECIMAL column precision is not lost along the entire
// DB <-> Go path.
//
// Verification items:
//   1. Driver type: whether DECIMAL is returned as string/[]byte rather than float64
//   2. json.Number read: precision preserved when scanning directly into json.Number
//   3. json.Number write: precision preserved when passing json.Number to db.Exec (implicit string conversion)
//   4. regobrick.Number: precision preserved when going through Number.Scan/Value
//   5. AllTypes: composite scan of DECIMAL + integer types (BIGINT, INTEGER)
//
// Running the tests:
//   cd tests/integration && go test -tags=integration -v -run TestDriver
//
// =============================================================================

const sentinelDecimal = "123456789012345678901234567890123456.78"
const testInteger = int64(9223372036854775807)

// Values for float64 precision testing
const testFloatInput = "123.45678901234567890" // input value (exceeds precision)
const testFloatExpected = "123.45678901234568" // expected value (float64 conversion result)

// =============================================================================
// DB setup helpers
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

	// Wait for MySQL to start
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
// Verification functions
// =============================================================================

// verifyDriverType verifies the Go type returned by the driver for a DECIMAL column.
// string/[]byte preserves precision, float64 loses precision.
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

// Stringer is a type that has a String() method
type Stringer interface {
	String() string
}

// verifyScan is a generic scan verification. T is json.Number or regobrick.Number
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

// verifyNumberValue verifies writing by passing regobrick.Number directly to db.Exec (driver.Valuer path)
// createTableSQL: table creation SQL (e.g. "CREATE TABLE test_value (val DECIMAL(50, 2))")
// placeholder: parameter placeholder (PostgreSQL: "$1", MySQL/SQLite: "?")
func verifyNumberValue(t *testing.T, db *sql.DB, createTableSQL, placeholder string) {
	_, err := db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Pass regobrick.Number directly -> driver.Valuer.Value() is called
	n := regobrick.Number(sentinelDecimal)
	_, err = db.Exec("INSERT INTO test_value (val) VALUES ("+placeholder+")", n)
	if err != nil {
		t.Fatalf("failed to insert via Number: %v", err)
	}

	// Read back as a raw string to verify
	var raw string
	if err := db.QueryRow("SELECT val FROM test_value").Scan(&raw); err != nil {
		t.Fatalf("failed to scan raw: %v", err)
	}

	if raw != sentinelDecimal {
		t.Fatalf("Number.Value precision lost: got %s, want %s", raw, sentinelDecimal)
	}
	t.Logf("PASS: Number.Value preserved precision")
}

// verifyJsonNumberValue verifies writing by passing json.Number directly to db.Exec (implicit string conversion path)
// Because json.Number is `type Number string`, it is not handled as a driver.Valuer but
// through the reflect.String case of DefaultParameterConverter.ConvertValue.
func verifyJsonNumberValue(t *testing.T, db *sql.DB, createTableSQL, placeholder string) {
	_, err := db.Exec(createTableSQL)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Pass json.Number directly -> DefaultParameterConverter converts it to string
	n := json.Number(sentinelDecimal)
	_, err = db.Exec("INSERT INTO test_jn_value (val) VALUES ("+placeholder+")", n)
	if err != nil {
		t.Fatalf("failed to insert via json.Number: %v", err)
	}

	// Read back as a raw string to verify
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
// PostgreSQL pgx tests
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

	// Verify driver type
	t.Run("DecimalDriverType", func(t *testing.T) {
		verifyDriverType(t, db, "SELECT dec_val FROM all_types")
	})

	// Scan all types with regobrick.Number (json.Number is excluded since it only supports string)
	t.Run("Number/DECIMAL", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT dec_val FROM all_types", sentinelDecimal, "regobrick.Number")
	})
	t.Run("Number/BIGINT", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT bigint_val FROM all_types", "9223372036854775807", "regobrick.Number")
	})
}

// =============================================================================
// PostgreSQL lib/pq tests
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
// MySQL tests
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

	// Verify driver type
	t.Run("DecimalDriverType", func(t *testing.T) {
		verifyDriverType(t, db, "SELECT dec_val FROM all_types")
	})

	// Scan all types with regobrick.Number (json.Number is excluded since it only supports string)
	t.Run("Number/DECIMAL", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT dec_val FROM all_types", sentinelDecimal, "regobrick.Number")
	})
	t.Run("Number/BIGINT", func(t *testing.T) {
		verifyScan[regobrick.Number](t, db, "SELECT bigint_val FROM all_types", "9223372036854775807", "regobrick.Number")
	})
}

// =============================================================================
// SQLite tests
// =============================================================================

func TestDriverIntegrity_SQLite(t *testing.T) {
	setup := setupSQLite(t)
	defer setup.cleanup()

	db := openDB(t, setup)
	defer db.Close()

	// SQLite has no DECIMAL type, so use TEXT
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

	// Verify driver type
	t.Run("TextDriverType", func(t *testing.T) {
		verifyDriverType(t, db, "SELECT text_val FROM all_types")
	})

	// Scan all types with regobrick.Number (json.Number is excluded since it only supports string)
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
		// Verify that a value exceeding precision matches the float64 conversion result
		verifyScan[regobrick.Number](t, db, "SELECT real_precision_val FROM all_types", testFloatExpected, "regobrick.Number")
	})
}
