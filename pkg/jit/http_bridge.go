package jit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
	"unsafe"
)

func nativeValue(v interface{}) uint64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case string:
		return uint64(nativeString(val))
	case []byte:
		return uint64(nativeString(string(val)))
	case int:
		return uint64(val)
	case int64:
		return uint64(val)
	case int32:
		return uint64(val)
	case int16:
		return uint64(val)
	case int8:
		return uint64(val)
	case uint:
		return uint64(val)
	case uint64:
		return val
	case uint32:
		return uint64(val)
	case uint16:
		return uint64(val)
	case uint8:
		return uint64(val)
	case float64:
		if val == float64(int64(val)) {
			return uint64(int64(val))
		}
		return uint64(nativeString(fmt.Sprintf("%v", val)))
	case float32:
		if val == float32(int64(val)) {
			return uint64(int64(val))
		}
		return uint64(nativeString(fmt.Sprintf("%v", val)))
	case bool:
		if val {
			return 1
		}
		return 0
	case time.Time:
		return uint64(nativeString(val.Format("2006-01-02 15:04:05")))
	case map[string]interface{}:
		return uint64(nativeMap(val))
	case []interface{}:
		return uint64(nativeSlice(val))
	default:
		return uint64(nativeString(fmt.Sprintf("%v", val)))
	}
}

func nativeSlice(items []interface{}) uintptr {
	count := len(items)
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
	for i, it := range items {
		val64 := nativeValue(it)
		*(*uint64)(unsafe.Add(dp, uintptr(i*8))) = val64
	}
	return arrPtr
}

func handleHttpJson(args ...uintptr) uintptr {
	if len(args) < 2 {
		return nativeMap(map[string]interface{}{})
	}
	httpMethod := strings.ToUpper(readCString(args[0]))
	if httpMethod == "" {
		httpMethod = "GET"
	}
	urlStr := readCString(args[1])
	if urlStr == "" {
		return nativeMap(map[string]interface{}{})
	}

	req, err := http.NewRequest(httpMethod, urlStr, nil)
	if err != nil {
		return nativeMap(map[string]interface{}{})
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Stremio/4.4 Joss/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nativeMap(map[string]interface{}{})
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nativeMap(map[string]interface{}{})
	}

	var data interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nativeMap(map[string]interface{}{})
	}

	return uintptr(nativeValue(data))
}

func handleHttpGet(args ...uintptr) uintptr {
	if len(args) < 1 {
		return nativeString("")
	}
	urlStr := readCString(args[0])
	if urlStr == "" {
		return nativeString("")
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nativeString("")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Stremio/4.4 Joss/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nativeString("")
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nativeString("")
	}
	return nativeString(string(bodyBytes))
}

func handleHttpRequest(args ...uintptr) uintptr {
	emptyResp := func() uintptr {
		return nativeMap(map[string]interface{}{
			"status":  0,
			"headers": map[string]interface{}{},
			"body":    "",
		})
	}

	if len(args) < 2 {
		return emptyResp()
	}
	httpMethod := strings.ToUpper(readCString(args[0]))
	if httpMethod == "" {
		httpMethod = "GET"
	}
	urlStr := readCString(args[1])
	if urlStr == "" {
		return emptyResp()
	}

	req, err := http.NewRequest(httpMethod, urlStr, nil)
	if err != nil {
		return emptyResp()
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Stremio/4.4 Joss/1.0")

	timeout := 4 * time.Second
	followRedirects := true

	if len(args) > 2 && args[2] != 0 {
		optsVal := readNativeValue(args[2])
		if optsMap, ok := optsVal.(map[string]interface{}); ok {
			if tVal, exists := optsMap["timeout"]; exists {
				switch tv := tVal.(type) {
				case int:
					if tv > 0 {
						timeout = time.Duration(tv) * time.Second
					}
				case int64:
					if tv > 0 {
						timeout = time.Duration(tv) * time.Second
					}
				case float64:
					if tv > 0 {
						timeout = time.Duration(tv * float64(time.Second))
					}
				}
			}
			if frVal, exists := optsMap["follow_redirects"]; exists {
				switch fv := frVal.(type) {
				case bool:
					followRedirects = fv
				case int64:
					followRedirects = (fv != 0)
				case int:
					followRedirects = (fv != 0)
				}
			}
			if hdrsVal, exists := optsMap["headers"]; exists {
				if hdrsMap, ok := hdrsVal.(map[string]interface{}); ok {
					for hk, hv := range hdrsMap {
						req.Header.Set(hk, fmt.Sprintf("%v", hv))
					}
				}
			}
		}
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !followRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return emptyResp()
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	hMap := make(map[string]interface{})
	for k, v := range resp.Header {
		if len(v) > 0 {
			hMap[k] = v[0]
			hMap[strings.ToLower(k)] = v[0]
		}
	}
	resMap := map[string]interface{}{
		"status":  resp.StatusCode,
		"headers": hMap,
		"body":    string(bodyBytes),
	}
	return uintptr(nativeValue(resMap))
}

func handleSystemRun(args ...uintptr) uintptr {
	if len(args) < 1 {
		return nativeString("")
	}
	cmdName := readCString(args[0])
	if cmdName == "" {
		return nativeString("")
	}
	var cmdArgs []string
	if len(args) > 1 && args[1] != 0 {
		rawArgs := readNativeArray(args[1])
		for _, a := range rawArgs {
			cmdArgs = append(cmdArgs, fmt.Sprintf("%v", a))
		}
	}
	cmd := exec.Command(cmdName, cmdArgs...)
	out, _ := cmd.CombinedOutput()
	return nativeString(string(out))
}

func readNativeValue(ptr uintptr) interface{} {
	if ptr == 0 {
		return nil
	}
	if ptr < 0x10000 {
		return int64(ptr)
	}

	p := toUnsafe(ptr)
	magic := *(*uint64)(p)
	if magic == jossMapMagic {
		m := make(map[string]interface{})
		for b := 0; b < 32768; b++ {
			nodePtr := *(*uintptr)(unsafe.Add(p, 16+b*8))
			for nodePtr != 0 {
				np := toUnsafe(nodePtr)
				key := readCString(*(*uintptr)(np))
				val64 := *(*uint64)(unsafe.Add(np, 8))
				m[key] = readNativeValue(uintptr(val64))
				nodePtr = *(*uintptr)(unsafe.Add(np, 16))
			}
		}
		return m
	}

	length := *(*int64)(p)
	capVal := *(*int64)(unsafe.Add(p, 8))
	dataPtr := *(*uintptr)(unsafe.Add(p, 16))
	if length >= 0 && capVal >= length && capVal > 0 && capVal < 1000000 && (dataPtr >= 0x10000 || length == 0) {
		arr := make([]interface{}, length)
		if dataPtr != 0 {
			dp := toUnsafe(dataPtr)
			for i := int64(0); i < length; i++ {
				elem := *(*uint64)(unsafe.Add(dp, uintptr(i*8)))
				arr[i] = readNativeValue(uintptr(elem))
			}
		}
		return arr
	}

	return readCString(ptr)
}

func handleJson(args ...uintptr) uintptr {
	if len(args) == 0 || args[0] == 0 {
		return nativeString("{}")
	}
	val := readNativeValue(args[0])
	b, err := json.Marshal(val)
	if err != nil {
		return nativeString("{}")
	}
	return nativeString(string(b))
}

func handleStrContains(args ...uintptr) uintptr {
	if len(args) < 2 {
		return 0
	}
	s1 := readCString(args[0])
	s2 := readCString(args[1])
	if strings.Contains(s1, s2) {
		return 1
	}
	return 0
}

func init() {
	RegisterHostBridgeHandler("Http::json", handleHttpJson)
	RegisterHostBridgeHandler("Http::get", handleHttpGet)
	RegisterHostBridgeHandler("Http::post", handleHttpJson)
	RegisterHostBridgeHandler("Http::request", handleHttpRequest)
	RegisterHostBridgeHandler("System::Run", handleSystemRun)
	RegisterHostBridgeHandler("json", handleJson)
	RegisterHostBridgeHandler("fn_json", handleJson)
	RegisterHostBridgeHandler("str_contains", handleStrContains)
	RegisterHostBridgeHandler("fn_str_contains", handleStrContains)
	RegisterHostBridgeHandler("contains", handleStrContains)
	RegisterHostBridgeHandler("fn_contains", handleStrContains)
}
