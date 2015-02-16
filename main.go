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
}

func NewLogRow(n int) *LogRow {
	lr := &LogRow{}

	lr.values = make([]string, n)

	for i := 0; i < n; i++ {
		lr.ivalues = append(lr.ivalues, interface{}(&lr.values[i]))
	}

	return lr
}

func (lr *LogRow) printRowsAsLogs(rows *sql.Rows) {
	var printedLineSkipped bool

	for rows.Next() {
		if err := rows.Scan(lr.ivalues...); err != nil {
			fail(err)
		}

		timeUnix, err := strconv.ParseInt(lr.values[8], 10, 64)
		if err != nil && !printedLineSkipped {
			fmt.Println(oneLineSkipped)
			continue
		}
		datetime := time.Unix(timeUnix, 0)

		str, err := formatPhpString(lr.values[7], lr.values[12])
		if err != nil && !printedLineSkipped {
			fmt.Println(oneLineSkipped)
			continue
		}

		printedLineSkipped = true
		fmt.Printf("%s %s\n", datetime.Format(time.RFC3339), str)
	}
}

func main() {
	if len(os.Args) < 6 {
		fail(fmt.Errorf("Usage: <user> <password> <host> <port> <database>"))
	}

	db, err := sql.Open("mysql", makeMysqlDSN(os.Args[1:6]))
	if err != nil {
		fail(err)
	}

	rows, err := db.Query("SELECT * FROM sys_log ORDER BY tstamp DESC LIMIT 200")
	if err != nil {
		fail(err)
	}

	defer rows.Close()

	lr := NewLogRow(16)
	lr.printRowsAsLogs(rows)

	// TODO: Update every second...

	os.Exit(0)
}
