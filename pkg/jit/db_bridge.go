package jit

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/jossecurity/joss/pkg/core"
)

var (
	dbBridgeMu sync.Mutex
	dbInstance *sql.DB
)

func getOrInitDB() *sql.DB {
	dbBridgeMu.Lock()
	defer dbBridgeMu.Unlock()
	if dbInstance != nil {
		if err := dbInstance.Ping(); err == nil {
			return dbInstance
		}
		_ = dbInstance.Close()
		dbInstance = nil
	}

	envRes := core.DetectAndLoadEnv(nil)
	driver := strings.ToLower(strings.TrimSpace(envRes.EnvMap["DB"]))
	if driver == "" {
		driver = strings.ToLower(strings.TrimSpace(envRes.EnvMap["DB_CONNECTION"]))
	}
	if driver == "" {
		driver = "mysql"
	}

	db, err := core.OpenConfiguredDatabase(driver, envRes.EnvMap)
	if err != nil {
		fmt.Printf("[HostBridge DB] Error de conexión (%s): %v\n", driver, err)
		return nil
	}
	if db != nil {
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)
		if err := db.Ping(); err != nil {
			fmt.Printf("[HostBridge DB] Error verificando conexión (%s): %v\n", driver, err)
			_ = db.Close()
			return nil
		}
		dbInstance = db
	}
	return dbInstance
}

func readCString(ptr uintptr) string {
	if ptr == 0 || !isReadablePointer(ptr) {
		return ""
	}
	p := toUnsafe(ptr)
	var buf []byte
	for i := 0; ; i++ {
		b := *(*byte)(unsafe.Add(p, i))
		if b == 0 {
			break
		}
		buf = append(buf, b)
		if len(buf) > 1000000 {
			break
		}
	}
	return string(buf)
}

func readNativeArray(arrPtr uintptr) []interface{} {
	if arrPtr == 0 {
		return nil
	}
	p := toUnsafe(arrPtr)
	length := *(*int64)(p)
	if length <= 0 || length > 10000 {
		return nil
	}
	dataPtr := *(*uintptr)(unsafe.Add(p, 16))
	if dataPtr == 0 {
		return nil
	}
	dp := toUnsafe(dataPtr)
	res := make([]interface{}, 0, length)
	for i := int64(0); i < length; i++ {
		val := *(*uint64)(unsafe.Add(dp, i*8))
		if val >= 0x10000 {
			res = append(res, readCString(uintptr(val)))
		} else {
			res = append(res, int64(val))
		}
	}
	return res
}

func nativeString(s string) uintptr {
	b := []byte(s)
	ptr := allocNative(len(b) + 1)
	if ptr == 0 {
		return 0
	}
	p := toUnsafe(ptr)
	for i, c := range b {
		*(*byte)(unsafe.Add(p, i)) = c
	}
	*(*byte)(unsafe.Add(p, len(b))) = 0
	return ptr
}

const (
	jossMapMagic = uint64(0x4A4F53534D415021) // "JOSSMAP!"
	fnvOffset    = uint64(0xCBF29CE484222325)
	fnvPrime     = uint64(0x100000001B3)
)

func fnvHash(s string) uint64 {
	h := fnvOffset
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return h
}

func nativeMap(row map[string]interface{}) uintptr {
	mapPtr := allocNative(262160)
	if mapPtr == 0 {
		return 0
	}
	mp := toUnsafe(mapPtr)
	*(*uint64)(mp) = jossMapMagic
	*(*int64)(unsafe.Add(mp, 8)) = int64(len(row))

	for k, v := range row {
		keyHash := fnvHash(k)
		bucketIdx := keyHash & 32767
		keyPtr := nativeString(k)
		val64 := nativeValue(v)

		nodePtr := allocNative(32)
		if nodePtr == 0 {
			continue
		}
		np := toUnsafe(nodePtr)
		*(*uintptr)(np) = keyPtr
		*(*uint64)(unsafe.Add(np, 8)) = val64
		oldHead := *(*uintptr)(unsafe.Add(mp, 16+bucketIdx*8))
		*(*uintptr)(unsafe.Add(np, 16)) = oldHead
		*(*uint64)(unsafe.Add(np, 24)) = keyHash
		*(*uintptr)(unsafe.Add(mp, 16+bucketIdx*8)) = nodePtr
	}
	return mapPtr
}

func nativeArray(rows []map[string]interface{}) uintptr {
	count := len(rows)
	arrCap := count
	if arrCap < 16 {
		arrCap = 16
	}
	arrPtr := allocNative(24)
	if arrPtr == 0 {
		return 0
	}
	dataPtr := allocNative(arrCap * 8)
	if dataPtr == 0 {
		return arrPtr
	}

	ap := toUnsafe(arrPtr)
	*(*int64)(ap) = int64(count)
	*(*int64)(unsafe.Add(ap, 8)) = int64(arrCap)
	*(*uintptr)(unsafe.Add(ap, 16)) = dataPtr

	dp := toUnsafe(dataPtr)
	for i, r := range rows {
		rowMapPtr := nativeMap(r)
		*(*uintptr)(unsafe.Add(dp, i*8)) = rowMapPtr
	}
	return arrPtr
}

func nativeEmptyArray() uintptr {
	return allocNative(24)
}

func handleDBIsConnected(args ...uintptr) uintptr {
	db := getOrInitDB()
	if db != nil && db.Ping() == nil {
		return 1
	}
	return 0
}

func handleDBQuery(args ...uintptr) uintptr {
	if len(args) < 1 {
		return nativeEmptyArray()
	}
	sqlStr := readCString(args[0])
	if sqlStr == "" {
		return nativeEmptyArray()
	}
	var bindings []interface{}
	if len(args) >= 2 && args[1] != 0 {
		bindings = readNativeArray(args[1])
	}

	db := getOrInitDB()
	if db == nil {
		return nativeEmptyArray()
	}

	// Execute query
	rows, err := db.Query(sqlStr, bindings...)
	if err != nil {
		return nativeEmptyArray()
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nativeEmptyArray()
	}

	var results []map[string]interface{}
	vals := make([]interface{}, len(cols))
	valPtrs := make([]interface{}, len(cols))
	for i := range cols {
		valPtrs[i] = &vals[i]
	}

	for rows.Next() {
		if err := rows.Scan(valPtrs...); err != nil {
			continue
		}
		row := make(map[string]interface{}, len(cols))
		for i, colName := range cols {
			val := vals[i]
			if b, ok := val.([]byte); ok {
				row[colName] = string(b)
			} else {
				row[colName] = val
			}
		}
		results = append(results, row)
	}

	return nativeArray(results)
}

func handleDBExecute(args ...uintptr) uintptr {
	if len(args) < 1 {
		return 0
	}
	sqlStr := readCString(args[0])
	if sqlStr == "" {
		return 0
	}
	var bindings []interface{}
	if len(args) >= 2 && args[1] != 0 {
		bindings = readNativeArray(args[1])
	}

	db := getOrInitDB()
	if db == nil {
		return 0
	}

	res, err := db.Exec(sqlStr, bindings...)
	if err != nil {
		return 0
	}

	trimmed := strings.ToUpper(strings.TrimSpace(sqlStr))
	if strings.HasPrefix(trimmed, "INSERT") {
		if id, err := res.LastInsertId(); err == nil && id > 0 {
			return uintptr(id)
		}
	}
	n, _ := res.RowsAffected()
	return uintptr(n)
}

func init() {
	RegisterHostBridgeHandler("__joss_db_is_connected", handleDBIsConnected)
	RegisterHostBridgeHandler("__joss_db_query", handleDBQuery)
	RegisterHostBridgeHandler("__joss_db_execute", handleDBExecute)
}
