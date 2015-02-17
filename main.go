package main

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/dullgiulio/go-php-serialize/phpserialize"
	_ "github.com/go-sql-driver/mysql"
)

const oneLineSkipped = "--- One line skipped ---"

func formatGetVerbs(fmtstr string) (string, int, error) {
	var insideVerb bool
	var verbs int

	str := make([]rune, 0)

	for _, r := range fmtstr {
		if r == '%' {
			if !insideVerb {
				insideVerb = true
				verbs++
			}

			str = append(str, '%')
			continue
		}

		if insideVerb {
			switch r {
			case 'v', 'T', 't', 'b', 'c', 'd', 'o', 'q', 'x', 'X',
				'U', 'e', 'E', 'f', 'F', 'g', 'G', 's', 'p':
				insideVerb = false
				str = append(str, 's')
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
				'+', '.', ' ', '-', '#', '[', ']', '*':
				continue // This is a flag
			default:
				return "", 0, fmt.Errorf("String format error") // TODO
			}
		} else {
			str = append(str, r)
		}
	}

	return string(str), verbs, nil
}

func phpArrayValues(phpMap map[interface{}]interface{}, ikeys []int, skeys []string) []interface{} {
	values := make([]interface{}, 0)

	for i := range ikeys {
		v, ok := phpMap[int64(i)]
		if ok {
			if v == nil {
				values = append(values, "(null)")
				continue
			}

			if _, ok := v.(string); ok {
				values = append(values, v)
				continue
			}

			if _, ok := v.(int64); ok {
				values = append(values, fmt.Sprintf("%d", v))
				continue
			}

			if _, ok := v.(int); ok {
				values = append(values, fmt.Sprintf("%d", v))
				continue
			}

			values = append(values, fmt.Sprintf("%v", v))
		}
	}

	for s := range skeys {
		v, ok := phpMap[s]
		if ok {
			values = append(values, v)
		}
	}

	return values
}

func sortedMapKeys(m map[interface{}]interface{}) ([]int, []string) {
	ikeys := make([]int, 0)
	skeys := make([]string, 0)

	for k := range m {
		if v, ok := k.(string); ok {
			if vi, err := strconv.ParseInt(v, 10, 32); err == nil {
				ikeys = append(ikeys, int(vi))
			} else {
				skeys = append(skeys, v)
			}
			continue
		}

		if v, ok := k.(int64); ok {
			ikeys = append(ikeys, int(v))
		}
	}

	sort.Ints(ikeys)
	sort.Strings(skeys)

	return ikeys, skeys
}

func formatPhpString(format, data string) (string, error) {
	if val, n, err := formatGetVerbs(format); err != nil {
		return format, err
	} else {
		if n == 0 {
			return format, nil
		}

		format = val
	}

	decodeRes, err := phpserialize.Decode(data)
	if err != nil {
		return format, err
	}

	phpMap, ok := decodeRes.(map[interface{}]interface{})
	if !ok {
		return format, nil
	}

	phpMapKeysInt, phpMapKeysStr := sortedMapKeys(phpMap)

	values := phpArrayValues(phpMap, phpMapKeysInt, phpMapKeysStr)
	return fmt.Sprintf(format, values...), nil
}

func makeMysqlDSN(args []string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", args[0], args[1], args[2], args[3], args[4])
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}

type LogRow struct {
	values  []string
	ivalues []interface{}
	verbose bool
}

func NewLogRow(n int) *LogRow {
	lr := &LogRow{}

	lr.values = make([]string, n)

	for i := 0; i < n; i++ {
		lr.ivalues = append(lr.ivalues, interface{}(&lr.values[i]))
	}

	return lr
}

func (lr *LogRow) printRow() {
	timeUnix, err := strconv.ParseInt(lr.values[8], 10, 64)
	if err != nil && !lr.verbose {
		fmt.Println(oneLineSkipped)
		return
	}
	datetime := time.Unix(timeUnix, 0)

	str, err := formatPhpString(lr.values[7], lr.values[12])
	if err != nil && !lr.verbose {
		fmt.Println(oneLineSkipped)
		return
	}

	lr.verbose = true

	ipAddress := lr.values[11]
	if ipAddress == "" {
		ipAddress = "-"
	}

	fmt.Printf("%s [%s] %s\n", ipAddress, datetime.Format("02/Jan/2006:15:04:05 -0700"), str)
}

type Tailer struct {
	lastID chan int64
	row	   *LogRow
	db	   *sql.DB
	tableName string
	index string
	order string
	query string
}

func NewTailer(db *sql.DB, limit int, initialID int64, tableName, index, order string) *Tailer {
	t := &Tailer{
		lastID: make(chan int64, 1), // Important: keep 1 as buffer size.
		row:	NewLogRow(16),
		tableName: tableName,
		index: index,
		order: order,
		db: db,
	}

	t.query = fmt.Sprintf("SELECT * FROM %s WHERE %s > ? ORDER BY %s ASC LIMIT %d", t.tableName, t.index, t.order, limit)
	t.lastID <- initialID

	return t
}

func (t *Tailer) Tail() {
	lastID := <-t.lastID

	rows, err := t.db.Query(t.query, lastID)
	if err != nil {
		fail(err)
	}

	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(t.row.ivalues...); err != nil {
			fail(err)
		}

		t.row.printRow()

		lastID, err = strconv.ParseInt(t.row.values[8], 10, 64)
		if err != nil {
			fail(err)
		}
	}

	t.lastID <- lastID
}

func determineStartValue(db *sql.DB, limit int, tableName, index, order string) (value int64) {
	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s DESC LIMIT 1 OFFSET %d", index, tableName, order, limit + 1)
	err := db.QueryRow(query).Scan(&value)

	switch {
    case err == sql.ErrNoRows:
            return 0
    case err != nil:
            fail(err)
    }

	return value
}

func main() {
	if len(os.Args) < 6 {
		fail(fmt.Errorf("Usage: <user> <password> <host> <port> <database>"))
	}

	db, err := sql.Open("mysql", makeMysqlDSN(os.Args[1:6]))
	if err != nil {
		fail(err)
	}

	startVal := determineStartValue(db, 200, "sys_log", "tstamp", "tstamp")

	t := NewTailer(db, 200, startVal, "sys_log", "tstamp", "tstamp")

	for {
		t.Tail()
		<-time.After(1 * time.Second)
	}

	os.Exit(0)
}
