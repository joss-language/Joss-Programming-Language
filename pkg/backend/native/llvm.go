package native

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jossecurity/joss/pkg/ir"
)

// LLVMEmitter traduce un ir.Program válido a LLVM IR textual (.ll).
type LLVMEmitter struct {
	stringConstants map[string]string // texto -> @.str.N
	nextStringID    int
	program         *ir.Program
	varargSyms      map[string]bool
	symRetTypes     map[string]string
	symArgTypes     map[string][]string
	tempLLVMTypes   map[int]string
	nextDPFnID      int64
	activeDPFnID    int64
	activeDPParam0  string
	activeDPParam1  string
	currentFn       *ir.Function
	currentFnHasTry bool
	TargetOS        string
	TargetArch      string
}

func NewLLVMEmitter() *LLVMEmitter {
	return &LLVMEmitter{
		stringConstants: make(map[string]string),
	}
}

func (e *LLVMEmitter) Emit(prog *ir.Program) (string, error) {
	if prog == nil {
		return "", fmt.Errorf("llvm emitter: programa nil")
	}
	e.program = prog

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("; ModuleID = '%s'\n", prog.Name))
	buf.WriteString("source_filename = \"" + prog.Name + ".joss\"\n\n")

	var bodyBuf bytes.Buffer
	if len(prog.Globals) > 0 {
		globalNames := make([]string, 0, len(prog.Globals))
		for name := range prog.Globals {
			globalNames = append(globalNames, name)
		}
		sort.Strings(globalNames)
		buf.WriteString("; Joss module globals\n")
		for _, name := range globalNames {
			global := prog.Globals[name]
			buf.WriteString(fmt.Sprintf("@joss_global_%s = internal global %s zeroinitializer, align 8\n", cleanIdent(name), MapLLVMType(global.Type)))
			if cleanIdent(name) == "__joss_request_params" {
				buf.WriteString("declare void @joss_set_active_request(i8*)\n")
			}
		}
		buf.WriteString("\n")
	}

	// Declarar funciones de runtime requeridas
	buf.WriteString("; Declaraciones de Runtime Nativo Joss\n")
	buf.WriteString("declare void @joss_print_i64(i64)\n")
	buf.WriteString("declare void @joss_print_f64(double)\n")
	buf.WriteString("declare void @joss_print_bool(i1)\n")
	buf.WriteString("declare void @joss_print_string(i8*)\n")
	buf.WriteString("declare void @joss_print_mixed(i64)\n")
	buf.WriteString("declare void @joss_panic(i8*)\n")
	buf.WriteString("declare i8* @joss_try_push()\n")
	buf.WriteString("declare void @joss_try_pop()\n")
	buf.WriteString("declare i8* @joss_catch_msg()\n")
	buf.WriteString("declare void @joss_throw(i8*)\n")
	if e.TargetOS == "windows" {
		if e.TargetArch == "arm64" {
			buf.WriteString("declare i32 @__intrinsic_setjmpex(i8*, i8*) returns_twice\n")
		} else {
			buf.WriteString("declare i32 @setjmp(i8*, i8*) returns_twice\n")
		}
	} else {
		buf.WriteString("declare i32 @setjmp(i8*) returns_twice\n")
	}
	buf.WriteString("declare i8* @joss_str_concat(i8*, i8*)\n")
	buf.WriteString("declare i8* @joss_obj_new(i8*)\n")
	buf.WriteString("declare void @joss_obj_set_field_i64(i8*, i8*, i64)\n")
	buf.WriteString("declare i64 @joss_obj_get_field_i64(i8*, i8*)\n")
	buf.WriteString("declare void @joss_obj_set_field_str(i8*, i8*, i8*)\n")
	buf.WriteString("declare i8* @joss_obj_get_field_str(i8*, i8*)\n")
	buf.WriteString("declare i8* @joss_arr_new(i64)\n")
	buf.WriteString("declare void @joss_arr_push_i64(i8*, i64)\n")
	buf.WriteString("declare void @joss_arr_push_str(i8*, i8*)\n")
	buf.WriteString("declare i64 @joss_arr_get_i64(i8*, i64)\n")
	buf.WriteString("declare i8* @joss_arr_get_str(i8*, i64)\n")
	buf.WriteString("declare void @joss_arr_set_i64(i8*, i64, i64)\n")
	buf.WriteString("declare void @joss_arr_set_str(i8*, i64, i8*)\n")
	buf.WriteString("declare i64 @joss_arr_len(i8*)\n")
	buf.WriteString("declare i8* @joss_map_new()\n")
	buf.WriteString("declare void @joss_map_set_i64(i8*, i8*, i64)\n")
	buf.WriteString("declare i64 @joss_map_get_i64(i8*, i8*)\n")
	buf.WriteString("declare void @joss_map_set_str(i8*, i8*, i8*)\n")
	buf.WriteString("declare i8* @joss_map_get_str(i8*, i8*)\n\n")
	buf.WriteString("declare void @joss_map_delete_i64(i8*, i8*)\n")
	buf.WriteString("declare i8* @joss_box_value(i64, i64)\n")
	buf.WriteString("declare i64 @joss_unbox_value(i64)\n")
	buf.WriteString("declare void @joss_closure_env_set(i8*, i8*)\n")
	buf.WriteString("declare i8* @joss_closure_env_get(i8*)\n")
	buf.WriteString("declare i8* @joss_closure_alloc(i8*, i8*)\n")
	buf.WriteString("declare i8* @joss_closure_prepare_call(i8*)\n")
	buf.WriteString("declare i1 @joss_is_not_null(i64)\n")
	buf.WriteString("declare i8* @joss_i64_to_str(i64)\n")
	buf.WriteString("declare i8* @joss_f64_to_string(double)\n")
	buf.WriteString("declare i8* @joss_buf_new(i64)\n")
	buf.WriteString("declare void @joss_buf_free(i8*)\n")
	buf.WriteString("declare i64 @joss_buf_len(i8*)\n")
	buf.WriteString("declare i8 @joss_buf_get(i8*, i64)\n")
	buf.WriteString("declare void @joss_buf_set(i8*, i64, i8)\n")
	buf.WriteString("declare void @joss_buf_append_byte(i8*, i8)\n")
	buf.WriteString("declare void @joss_buf_append_cstr(i8*, i8*)\n")
	buf.WriteString("declare i8* @joss_buf_to_cstr(i8*)\n")
	buf.WriteString("declare i8* @joss_sb_new()\n")
	buf.WriteString("declare void @joss_sb_append(i8*, i8*)\n")
	buf.WriteString("declare void @joss_sb_append_i64(i8*, i64)\n")
	buf.WriteString("declare i8* @joss_sb_to_string(i8*)\n")
	buf.WriteString("declare i64 @joss_intval(i64)\n")
	buf.WriteString("declare i64 @joss_abs(i64)\n")
	buf.WriteString("declare i64 @joss_min(i64, i64)\n")
	buf.WriteString("declare i64 @joss_max(i64, i64)\n")
	buf.WriteString("declare i64 @joss_strlen(i8*)\n")
	buf.WriteString("declare i1 @joss_str_contains(i8*, i8*)\n")
	buf.WriteString("declare i1 @joss_str_eq(i8*, i8*)\n")
	buf.WriteString("declare i1 @joss_dp_get(i64, i64, i64, i64*)\n")
	buf.WriteString("declare void @joss_dp_put(i64, i64, i64, i64)\n")
	buf.WriteString("declare i64 @joss_async(i64)\n")
	buf.WriteString("declare i64 @joss_await(i64)\n")
	buf.WriteString("declare i64 @joss_make_chan(i64)\n")
	buf.WriteString("declare void @joss_send(i64, i64)\n")
	buf.WriteString("declare i64 @joss_recv(i64)\n")
	buf.WriteString("declare void @joss_close(i64)\n")

	fixedDecls := map[string]bool{
		"joss_print_i64": true, "joss_print_f64": true, "joss_print_bool": true,
		"joss_print_string": true, "joss_print_mixed": true, "joss_panic": true,
		"joss_str_concat": true, "joss_obj_new": true,
		"joss_obj_set_field_i64": true, "joss_obj_get_field_i64": true,
		"joss_obj_set_field_str": true, "joss_obj_get_field_str": true,
		"joss_arr_new": true, "joss_arr_push_i64": true, "joss_arr_push_str": true,
		"joss_arr_get_i64": true, "joss_arr_get_str": true,
		"joss_arr_set_i64": true, "joss_arr_set_str": true, "joss_arr_len": true,
		"joss_map_new": true, "joss_map_set_i64": true, "joss_map_get_i64": true,
		"joss_map_set_str": true, "joss_map_get_str": true, "joss_map_delete_i64": true,
		"joss_box_value": true, "joss_unbox_value": true,
		"joss_closure_env_set": true, "joss_closure_env_get": true,
		"joss_closure_alloc": true, "joss_closure_prepare_call": true,
		"joss_is_not_null": true, "joss_i64_to_str": true, "joss_f64_to_string": true,
		"joss_buf_new": true, "joss_buf_free": true, "joss_buf_len": true,
		"joss_buf_get": true, "joss_buf_set": true,
		"joss_buf_append_byte": true, "joss_buf_append_cstr": true, "joss_buf_to_cstr": true,
		"joss_sb_new": true, "joss_sb_append": true, "joss_sb_append_i64": true, "joss_sb_to_string": true,
		"joss_intval": true, "joss_abs": true, "joss_min": true, "joss_max": true,
		"joss_strlen": true, "joss_str_contains": true, "joss_str_eq": true,
		"joss_dp_get": true, "joss_dp_put": true,
		"joss_async": true, "joss_await": true,
		"joss_make_chan": true, "joss_send": true, "joss_recv": true, "joss_close": true,
	}

	e.varargSyms = make(map[string]bool)
	e.symRetTypes = map[string]string{
		"joss_intval": "i64", "joss_abs": "i64", "joss_min": "i64", "joss_max": "i64",
		"joss_strlen": "i64", "joss_str_contains": "i1", "joss_str_eq": "i1",
		"joss_print_i64": "void", "joss_print_f64": "void", "joss_print_bool": "void",
		"joss_print_string": "void", "joss_print_mixed": "void", "joss_panic": "void",
		"joss_str_concat":      "i8*",
		"joss_closure_env_set": "void", "joss_closure_env_get": "i8*",
		"joss_closure_alloc": "i8*", "joss_closure_prepare_call": "i8*",
		"joss_is_not_null": "i1", "joss_i64_to_str": "i8*", "joss_f64_to_string": "i8*",
		"joss_dp_get": "i1", "joss_dp_put": "void",
		"joss_async": "i64", "joss_await": "i64",
		"joss_make_chan": "i64", "joss_send": "void", "joss_recv": "i64", "joss_close": "void",
	}
	e.symArgTypes = map[string][]string{
		"joss_intval":               {"i64"},
		"joss_abs":                  {"i64"},
		"joss_min":                  {"i64", "i64"},
		"joss_max":                  {"i64", "i64"},
		"joss_strlen":               {"i8*"},
		"joss_str_contains":         {"i8*", "i8*"},
		"joss_str_eq":               {"i8*", "i8*"},
		"joss_print_i64":            {"i64"},
		"joss_print_f64":            {"double"},
		"joss_print_bool":           {"i1"},
		"joss_print_string":         {"i8*"},
		"joss_print_mixed":          {"i64"},
		"joss_panic":                {"i8*"},
		"joss_str_concat":           {"i8*", "i8*"},
		"joss_closure_env_set":      {"i8*", "i8*"},
		"joss_closure_env_get":      {"i8*"},
		"joss_closure_alloc":        {"i8*", "i8*"},
		"joss_closure_prepare_call": {"i8*"},
		"joss_is_not_null":          {"i64"},
		"joss_i64_to_str":           {"i64"},
		"joss_f64_to_string":        {"double"},
		"joss_async":                {"i64"},
		"joss_await":                {"i64"},
		"joss_make_chan":            {"i64"},
		"joss_send":                 {"i64", "i64"},
		"joss_recv":                 {"i64"},
		"joss_close":                {"i64"},
	}

	knownExternalSigs := map[string]struct {
		ret  string
		args []string
	}{
		"joss_set_active_session":             {ret: "void", args: []string{"i8*"}},
		"joss_get_active_session":             {ret: "i8*", args: nil},
		"joss_get_active_request":             {ret: "i8*", args: nil},
		"joss_register_plugin_dispatcher":     {ret: "void", args: []string{"i8*"}},
		"joss___joss_http_dispatch":           {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"joss_router_register_middleware":     {ret: "i64", args: []string{"i8*", "i64"}},
		"joss_router_list_custom_middlewares": {ret: "i8*", args: nil},
		"fn_Schema_create":                    {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_Schema_table":                     {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_Schema_drop":                      {ret: "i64", args: []string{"i8*"}},
		"fn_Schema_dropIfExists":              {ret: "i64", args: []string{"i8*"}},
		"fn_Schema_rename":                    {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_Schema_hasTable":                  {ret: "i64", args: []string{"i8*"}},
		"fn_Schema_hasColumn":                 {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_Plugin_call":                      {ret: "i64", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Plugin_path":                      {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Plugin_platform":                  {ret: "i8*", args: nil},
		"fn_Plugin_stream":                    {ret: "i64", args: []string{"i8*", "i8*", "i8*"}},
		"fn_System_sleep":                     {ret: "i64", args: []string{"i64"}},
		"fn_System_env":                       {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_System_log":                       {ret: "void", args: []string{"i64"}},
		"fn_System_now":                       {ret: "i8*", args: []string{"i64"}},
		"fn_Process_constructor":              {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Process_start":                    {ret: "i64", args: []string{"i8*"}},
		"fn_Process_wait":                     {ret: "i64", args: []string{"i8*"}},
		"fn_Process_kill":                     {ret: "i64", args: []string{"i8*"}},
		"fn_Process_pid":                      {ret: "i64", args: []string{"i8*"}},
		"fn_Cron_schedule":                    {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Cron_runDue":                      {ret: "i64", args: nil},
		"fn_WebSocket_publish":                {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_WebSocket_subscriberCount":        {ret: "i64", args: []string{"i8*"}},
		"fn_Cache_put":                        {ret: "i64", args: []string{"i8*", "i64", "i64"}},
		"fn_Cache_has":                        {ret: "i64", args: []string{"i8*"}},
		"fn_Cache_get":                        {ret: "i64", args: []string{"i8*", "i64"}},
		"fn_Cache_forget":                     {ret: "i64", args: []string{"i8*"}},
		"fn_Markdown_readFile":                {ret: "i8*", args: []string{"i8*"}},
		"fn_Markdown_toHtml":                  {ret: "i8*", args: []string{"i8*"}},
		"fn_Response_json":                    {ret: "i8*", args: []string{"i64", "i64"}},
		"fn_Response_raw":                     {ret: "i8*", args: []string{"i8*", "i64", "i8*"}},
		"fn_Response_error":                   {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Response_download":                {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Response_stream":                  {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Response_redirect":                {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Response_back":                    {ret: "i8*", args: nil},
		"fn_WebResponse_with":                 {ret: "i8*", args: []string{"i8*", "i8*", "i64"}},
		"fn_WebResponse_withCookie":           {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"with":                                {ret: "i8*", args: []string{"i8*", "i8*", "i64"}},
		"withCookie":                          {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Http_request":                     {ret: "i8*", args: []string{"i8*", "i8*", "i8*", "i8*"}},
		"fn_Http_get":                         {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Http_post":                        {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Http_json":                        {ret: "i64", args: []string{"i8*", "i8*", "i64", "i8*"}},
		"joss_ws_wrap":                        {ret: "i8*", args: []string{"i8*"}},
		"joss_ws_obj_send":                    {ret: "i64", args: []string{"i8*", "i8*"}},
		"joss_ws_obj_on_message":              {ret: "i64", args: []string{"i8*", "i64"}},
		"joss_ws_obj_on_close":                {ret: "i64", args: []string{"i8*", "i64"}},
		"joss_ws_obj_subscribe":               {ret: "i64", args: []string{"i8*", "i8*"}},
		"joss_ws_obj_close":                   {ret: "i64", args: []string{"i8*"}},
		"joss_ws_run_loop":                    {ret: "i8*", args: []string{"i8*"}},
		"fn_Sitemap_add":                      {ret: "void", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Sitemap_provider":                 {ret: "i8*", args: []string{"i8*"}},
		"fn_Sitemap_exclude":                  {ret: "void", args: []string{"i8*"}},
		"fn_Sitemap_xsl":                      {ret: "i8*", args: nil},
		"fn_DateInterval_days":                {ret: "i8*", args: []string{"i64"}},
		"fn_DateInterval_hours":               {ret: "i8*", args: []string{"i64"}},
		"fn_DateInterval_minutes":             {ret: "i8*", args: []string{"i64"}},
		"fn_DateInterval_seconds":             {ret: "i8*", args: []string{"i64"}},
		"fn_DateInterval_totalSeconds":        {ret: "i64", args: []string{"i8*"}},
		"fn_DateTime_now":                     {ret: "i8*", args: nil},
		"fn_DateTime_create":                  {ret: "i8*", args: []string{"i64", "i64", "i64"}},
		"fn_DateTime_parse":                   {ret: "i8*", args: []string{"i8*"}},
		"fn_DateTime_timestamp":               {ret: "i64", args: []string{"i8*"}},
		"fn_DateTime_format":                  {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_DateTime_iso":                     {ret: "i8*", args: []string{"i8*"}},
		"fn_DateTime_year":                    {ret: "i64", args: []string{"i8*"}},
		"fn_DateTime_month":                   {ret: "i64", args: []string{"i8*"}},
		"fn_DateTime_day":                     {ret: "i64", args: []string{"i8*"}},
		"fn_DateTime_hour":                    {ret: "i64", args: []string{"i8*"}},
		"fn_DateTime_minute":                  {ret: "i64", args: []string{"i8*"}},
		"fn_DateTime_second":                  {ret: "i64", args: []string{"i8*"}},
		"fn_DateTime_add":                     {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_DateTime_sub":                     {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_DateTime_diff":                    {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_FileStream_open":                  {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_FileStream_read":                  {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_FileStream_write":                 {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_FileStream_seek":                  {ret: "i64", args: []string{"i8*", "i64", "i64"}},
		"fn_FileStream_size":                  {ret: "i64", args: []string{"i8*"}},
		"fn_FileStream_flush":                 {ret: "void", args: []string{"i8*"}},
		"fn_FileStream_close":                 {ret: "void", args: []string{"i8*"}},
		"fn_StreamReader_open":                {ret: "i8*", args: []string{"i8*"}},
		"fn_StreamReader_readLine":            {ret: "i8*", args: []string{"i8*"}},
		"fn_StreamReader_readToEnd":           {ret: "i8*", args: []string{"i8*"}},
		"fn_StreamReader_close":               {ret: "void", args: []string{"i8*"}},
		"fn_StreamWriter_open":                {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_StreamWriter_write":               {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_StreamWriter_writeLine":           {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_StreamWriter_flush":               {ret: "void", args: []string{"i8*"}},
		"fn_StreamWriter_close":               {ret: "void", args: []string{"i8*"}},
		"fn_Sitemap_generate":                 {ret: "i8*", args: []string{"i8*"}},
		"fn_IndexNow_key":                     {ret: "i8*", args: nil},
		"fn_IndexNow_submit":                  {ret: "i64", args: []string{"i8*"}},
		"fn_SmtpClient_send":                  {ret: "i64", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Mail_send":                        {ret: "i64", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Zip_extract":                      {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_Model_registerClass":              {ret: "void", args: []string{"i8*", "i8*"}},
		"fn_Model_registerMethod":             {ret: "void", args: []string{"i8*", "i8*", "i8*", "i64"}},
		"fn_Model_newInstance":                {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_query":                      {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_all":                        {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_create":                     {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Model_firstOrNew":                 {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Model_firstOrCreate":              {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Model_updateOrCreate":             {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Model_getAttr":                    {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_Model_setAttr":                    {ret: "void", args: []string{"i8*", "i8*", "i64"}},
		"fn_Model_fill":                       {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Model_forceFill":                  {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Model_where":                      {ret: "i8*", args: []string{"i8*", "i8*", "i64", "i64", "i64"}},
		"fn_Model_orWhere":                    {ret: "i8*", args: []string{"i8*", "i8*", "i64", "i64", "i64"}},
		"fn_Model_whereIn":                    {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Model_whereNull":                  {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Model_whereNotNull":               {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Model_orderBy":                    {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Model_orderByAsc":                 {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Model_orderByDesc":                {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Model_limit":                      {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Model_offset":                     {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Model_withTrashed":                {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_onlyTrashed":                {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_withoutTrashed":             {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_withoutGlobalScope":         {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Model_withoutGlobalScopes":        {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_scope":                      {ret: "i8*", args: []string{"i8*", "i8*", "i64", "i64"}},
		"fn_Model_with":                       {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Model_load":                       {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Model_loadMissing":                {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Model_get":                        {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_first":                      {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_find":                       {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Model_count":                      {ret: "i64", args: []string{"i8*"}},
		"fn_Model_exists":                     {ret: "i64", args: []string{"i8*"}},
		"fn_Model_save":                       {ret: "i64", args: []string{"i8*"}},
		"fn_Model_refresh":                    {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_delete":                     {ret: "i64", args: []string{"i8*"}},
		"fn_Model_forceDelete":                {ret: "i64", args: []string{"i8*"}},
		"fn_Model_restore":                    {ret: "i64", args: []string{"i8*"}},
		"fn_Model_isDirty":                    {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_Model_isClean":                    {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_Model_wasChanged":                 {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_Model_getOriginal":                {ret: "i64", args: []string{"i8*", "i8*"}},
		"fn_Model_getChanges":                 {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_toMap":                      {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_toJSON":                     {ret: "i8*", args: []string{"i8*"}},
		"fn_Model_hasOne":                     {ret: "i8*", args: []string{"i8*", "i8*", "i8*", "i8*"}},
		"fn_Model_hasMany":                    {ret: "i8*", args: []string{"i8*", "i8*", "i8*", "i8*"}},
		"fn_Model_belongsTo":                  {ret: "i8*", args: []string{"i8*", "i8*", "i8*", "i8*"}},
		"fn_Model_belongsToMany":              {ret: "i8*", args: []string{"i8*", "i8*", "i8*", "i8*", "i8*", "i8*", "i8*"}},
		"fn_Model_attach":                     {ret: "i64", args: []string{"i8*", "i64"}},
		"fn_Model_detach":                     {ret: "i64", args: []string{"i8*", "i64", "i64"}},
		"fn_Model_sync":                       {ret: "i64", args: []string{"i8*", "i64"}},
		"fn_Window_open":                      {ret: "i8*", args: []string{"i8*", "i64", "i64"}},
		"fn_Window_setTitle":                  {ret: "void", args: []string{"i8*", "i8*"}},
		"fn_Window_setSize":                   {ret: "void", args: []string{"i8*", "i64", "i64"}},
		"fn_Window_setMinSize":                {ret: "void", args: []string{"i8*", "i64", "i64"}},
		"fn_Window_setResizable":              {ret: "void", args: []string{"i8*", "i64"}},
		"fn_Window_setFrameless":              {ret: "void", args: []string{"i8*", "i64"}},
		"fn_Window_setAlwaysOnTop":            {ret: "void", args: []string{"i8*", "i64"}},
		"fn_Window_setTheme":                  {ret: "void", args: []string{"i8*", "i8*"}},
		"fn_Window_navigate":                  {ret: "void", args: []string{"i8*", "i8*"}},
		"fn_Window_setHtml":                   {ret: "void", args: []string{"i8*", "i8*"}},
		"fn_Window_eval":                      {ret: "void", args: []string{"i8*", "i8*"}},
		"fn_Window_bind":                      {ret: "void", args: []string{"i8*", "i8*", "i64"}},
		"fn_Window_dispatch":                  {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Window_maximize":                  {ret: "void", args: []string{"i8*"}},
		"fn_Window_minimize":                  {ret: "void", args: []string{"i8*"}},
		"fn_Window_show":                      {ret: "void", args: []string{"i8*"}},
		"fn_Window_hide":                      {ret: "void", args: []string{"i8*"}},
		"fn_Window_close":                     {ret: "void", args: []string{"i8*"}},
		"fn_Window_run":                       {ret: "void", args: []string{"i8*"}},
		"fn_Window_pollEvents":                {ret: "i64", args: []string{"i8*"}},
		"fn_Window_render":                    {ret: "void", args: []string{"i8*", "i8*"}},
		"fn_Window_on":                        {ret: "void", args: []string{"i8*", "i8*", "i64"}},
		"fn_Window_getTitle":                  {ret: "i8*", args: []string{"i8*"}},
		"fn_Window_getWidth":                  {ret: "i64", args: []string{"i8*"}},
		"fn_Window_getHeight":                 {ret: "i64", args: []string{"i8*"}},
		"fn_Window_isVisible":                 {ret: "i64", args: []string{"i8*"}},
		"fn_Blueprint_id":                     {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_increments":             {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_integer":                {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_tinyInteger":            {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_smallInteger":           {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_mediumInteger":          {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_bigInteger":             {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_unsignedInteger":        {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_unsignedBigInteger":     {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_float":                  {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_double":                 {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_decimal":                {ret: "i8*", args: []string{"i8*", "i8*", "i64", "i64"}},
		"fn_Blueprint_char":                   {ret: "i8*", args: []string{"i8*", "i8*", "i64"}},
		"fn_Blueprint_string":                 {ret: "i8*", args: []string{"i8*", "i8*", "i64"}},
		"fn_Blueprint_text":                   {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_mediumText":             {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_longText":               {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_date":                   {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_dateTime":               {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_time":                   {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_timestamp":              {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_timestamps":             {ret: "i8*", args: []string{"i8*"}},
		"fn_Blueprint_softDeletes":            {ret: "i8*", args: []string{"i8*"}},
		"fn_Blueprint_boolean":                {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_json":                   {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_enum":                   {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Blueprint_nullable":               {ret: "i8*", args: []string{"i8*"}},
		"fn_Blueprint_unsigned":               {ret: "i8*", args: []string{"i8*"}},
		"fn_Blueprint_unique":                 {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Blueprint_default":                {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Blueprint_comment":                {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_dropColumn":             {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Blueprint_renameColumn":           {ret: "i8*", args: []string{"i8*", "i8*", "i8*"}},
		"fn_Blueprint_index":                  {ret: "i8*", args: []string{"i8*", "i64", "i8*"}},
		"fn_Blueprint_uniqueIndex":            {ret: "i8*", args: []string{"i8*", "i64", "i8*"}},
		"fn_Blueprint_dropIndex":              {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_foreign":                {ret: "i8*", args: []string{"i8*", "i64", "i8*"}},
		"fn_Blueprint_references":             {ret: "i8*", args: []string{"i8*", "i64"}},
		"fn_Blueprint_on":                     {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_onDelete":               {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_onUpdate":               {ret: "i8*", args: []string{"i8*", "i8*"}},
		"fn_Blueprint_dropForeign":            {ret: "i8*", args: []string{"i8*", "i8*"}},
	}

	// Pre-escanear instrucciones para declarar símbolos C externos, builtins y host services
	externDecls := make(map[string]string)
	registerCallDecl := func(sym string, dest *ir.TempValue, retT ir.Type, args []ir.Value, argTypes []ir.Type) {
		if fixedDecls[sym] || prog.Functions[sym] != nil {
			return
		}
		if known, ok := knownExternalSigs[sym]; ok {
			e.symRetTypes[sym] = known.ret
			e.symArgTypes[sym] = append([]string(nil), known.args...)
			externDecls[sym] = fmt.Sprintf("declare %s @%s(%s)\n", known.ret, sym, strings.Join(known.args, ", "))
			return
		}
		retStr := MapLLVMType(retT)
		if dest != nil && dest.ValueType.Kind != ir.TypeKindVoid {
			retStr = MapLLVMType(dest.ValueType)
		}
		argTs := make([]string, len(args))
		for idx, a := range args {
			if idx < len(argTypes) {
				argTs[idx] = MapLLVMType(argTypes[idx])
			} else {
				argTs[idx] = MapLLVMType(a.Type())
			}
		}
		sig := fmt.Sprintf("declare %s @%s(%s)\n", retStr, sym, strings.Join(argTs, ", "))
		if prev, seen := externDecls[sym]; seen {
			if prev != sig {
				prevRet := e.symRetTypes[sym]
				if prevRet == "void" && retStr != "void" {
					e.symRetTypes[sym] = retStr
				} else if prevRet != retStr && retStr != "void" {
					e.symRetTypes[sym] = "i64"
				}
				prevArgs := e.symArgTypes[sym]
				maxLen := len(prevArgs)
				if len(argTs) > maxLen {
					maxLen = len(argTs)
				}
				merged := make([]string, maxLen)
				for idx := 0; idx < maxLen; idx++ {
					switch {
					case idx < len(prevArgs) && idx < len(argTs):
						if prevArgs[idx] == argTs[idx] {
							merged[idx] = prevArgs[idx]
						} else {
							merged[idx] = "i64"
						}
					case idx < len(prevArgs):
						merged[idx] = prevArgs[idx]
					default:
						merged[idx] = argTs[idx]
					}
				}
				e.symArgTypes[sym] = merged
				externDecls[sym] = fmt.Sprintf("declare %s @%s(%s)\n", e.symRetTypes[sym], sym, strings.Join(merged, ", "))
			}
			return
		}
		e.symRetTypes[sym] = retStr
		e.symArgTypes[sym] = argTs
		externDecls[sym] = sig
	}
	var scanDecls func([]*ir.BasicBlock)
	scanDecls = func(blocks []*ir.BasicBlock) {
		for _, block := range blocks {
			for _, inst := range block.Instructions {
				switch ci := inst.(type) {
				case *ir.ExternCallInst:
					registerCallDecl(ci.Symbol, ci.Dest, ci.RetType, ci.Args, ci.ArgTypes)
				case *ir.CallInst:
					if prog.Functions[ci.Callee] == nil {
						retT := ir.TypeVoid
						if ci.Dest != nil {
							retT = ci.Dest.ValueType
						}
						registerCallDecl(ci.Callee, ci.Dest, retT, ci.Args, nil)
					}
				case *ir.CallRuntimeInst:
					rtFunc := ci.Func
					if !strings.HasPrefix(rtFunc, "joss_") {
						rtFunc = "joss_" + rtFunc
					}
					if (rtFunc == "joss_to_string" || rtFunc == "joss_strval") && len(ci.Args) == 1 &&
						(ci.Args[0].Type().Kind == ir.TypeKindBool || ci.Args[0].Type().IsFloat() || ci.Args[0].Type().IsInteger()) {
						continue
					}
					retT := ir.TypeVoid
					if ci.Dest != nil {
						retT = ci.Dest.ValueType
					}
					registerCallDecl(rtFunc, ci.Dest, retT, ci.Args, nil)
				case *ir.CallBuiltinInst:
					if prog.Functions[ci.BuiltinID] != nil {
						continue
					}
					if ci.BuiltinID == "intval" && len(ci.Args) == 1 && (ci.Args[0].Type().IsFloat() || ci.Args[0].Type().IsInteger() || ci.Args[0].Type().Kind == ir.TypeKindBool) {
						continue
					}
					if (ci.BuiltinID == "floatval" || ci.BuiltinID == "decimal") && len(ci.Args) == 1 && (ci.Args[0].Type().IsInteger() || ci.Args[0].Type().IsFloat()) {
						continue
					}
					if (ci.BuiltinID == "strval" || ci.BuiltinID == "to_string") && len(ci.Args) == 1 &&
						(ci.Args[0].Type().Kind == ir.TypeKindBool || ci.Args[0].Type().IsFloat() || ci.Args[0].Type().IsInteger()) {
						continue
					}
					rtFunc := "joss_" + ci.BuiltinID
					retT := ir.TypeVoid
					if ci.Dest != nil {
						retT = ci.Dest.ValueType
					}
					args := normalizeLLVMBuiltinArgs(ci.BuiltinID, ci.Args)
					registerCallDecl(rtFunc, ci.Dest, retT, args, nil)
				case *ir.CallHostServiceInst:
					fnName := "fn_" + ci.ClassName + "_" + ci.MethodName
					args := ci.Args
					if hasJossHostServiceImplementation(prog, ci.ClassName, ci.MethodName) {
						fnName = jossHostServiceFunctionName(prog, ci.ClassName, ci.MethodName)
						if prog.Functions[fnName] != nil {
							continue
						}
					} else {
						args = normalizeLLVMHostServiceArgs(ci.ClassName, ci.MethodName, args)
					}
					retT := ir.TypeVoid
					if ci.Dest != nil {
						retT = ci.Dest.ValueType
					}
					registerCallDecl(fnName, ci.Dest, retT, args, nil)
				case *ir.CallPrimitiveMethodInst:
					rtFunc := resolveLLVMPrimitiveMethodSym(ci.TypeKind, ci.MethodName)
					retT := ir.TypeVoid
					if ci.Dest != nil {
						retT = ci.Dest.ValueType
					}
					registerCallDecl(rtFunc, ci.Dest, retT, normalizePrimitiveMethodArgs(ci.MethodName, ci.Args), nil)
				case *ir.TryCatchInst:
					scanDecls(ci.TryBlocks)
					scanDecls(ci.CatchBlocks)
				}
			}
		}
	}
	for _, fn := range prog.Functions {
		scanDecls(fn.Blocks)
	}
	if len(externDecls) > 0 {
		extKeys := make([]string, 0, len(externDecls))
		for k := range externDecls {
			extKeys = append(extKeys, k)
		}
		sort.Strings(extKeys)
		for _, k := range extKeys {
			buf.WriteString(externDecls[k])
		}
	}
	buf.WriteString("\n")

	// Emitir funciones de usuario
	fnNames := make([]string, 0, len(prog.Functions))
	for name := range prog.Functions {
		fnNames = append(fnNames, name)
	}
	sort.Strings(fnNames)

	for _, name := range fnNames {
		fn := prog.Functions[name]
		if err := e.emitFunction(&bodyBuf, fn); err != nil {
			return "", err
		}
		bodyBuf.WriteString("\n")
	}

	if len(e.stringConstants) > 0 {
		buf.WriteString("; String Constants\n")
		keys := make([]string, 0, len(e.stringConstants))
		for k := range e.stringConstants {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sym := e.stringConstants[k]
			escaped, length := formatLLVMStringLiteral(k)
			buf.WriteString(fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\", align 1\n", sym, length, escaped))
		}
		buf.WriteString("\n")
	}
	buf.Write(bodyBuf.Bytes())

	return buf.String(), nil
}

func (e *LLVMEmitter) registerString(val string) string {
	if sym, ok := e.stringConstants[val]; ok {
		return sym
	}
	sym := fmt.Sprintf("@.str.%d", e.nextStringID)
	e.nextStringID++
	e.stringConstants[val] = sym
	return sym
}

// isAutoDPEligible returns true when fn is a pure self-recursive function
// taking 1 or 2 i64 parameters and returning i64, making automatic memoization
// (Dynamic Programming) 100% semantically sound.
func isAutoDPEligible(fn *ir.Function) bool {
	if fn == nil || fn.Name == "main" || fn.ReturnType.Kind != ir.TypeKindI64 {
		return false
	}
	if len(fn.Params) < 1 || len(fn.Params) > 2 {
		return false
	}
	for _, p := range fn.Params {
		if p.ValueType.Kind != ir.TypeKindI64 {
			return false
		}
	}
	hasSelfCall := false
	for _, blk := range fn.Blocks {
		for _, inst := range blk.Instructions {
			switch i := inst.(type) {
			case *ir.AllocaInst, *ir.LoadInst, *ir.StoreInst, *ir.MoveInst,
				*ir.BinaryInst, *ir.UnaryInst, *ir.CompareInst, *ir.CastInst:
				// Pure local computation
			case *ir.CallInst:
				if i.Callee != fn.Name {
					return false
				}
				hasSelfCall = true
			default:
				return false
			}
		}
	}
	return hasSelfCall
}

func fnContainsTryCatch(blocks []*ir.BasicBlock) bool {
	for _, b := range blocks {
		for _, inst := range b.Instructions {
			if _, ok := inst.(*ir.TryCatchInst); ok {
				return true
			}
		}
	}
	return false
}

func (e *LLVMEmitter) emitFunction(buf *bytes.Buffer, fn *ir.Function) error {
	e.currentFn = fn
	e.currentFnHasTry = fnContainsTryCatch(fn.Blocks)
	e.tempLLVMTypes = make(map[int]string)
	e.activeDPFnID = 0
	e.activeDPParam0 = "0"
	e.activeDPParam1 = "0"
	retType := MapLLVMType(fn.ReturnType)

	// Parámetros
	paramStrs := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		paramStrs[i] = fmt.Sprintf("%s %s", MapLLVMType(p.ValueType), e.formatValue(p))
	}

	linkage := ""
	if fn.Name != "main" && !strings.HasPrefix(fn.Name, "joss_export_") {
		linkage = "internal "
	}
	buf.WriteString(fmt.Sprintf("define %s%s @%s(%s) {\n", linkage, retType, fn.Name, strings.Join(paramStrs, ", ")))

	if isAutoDPEligible(fn) && len(fn.Blocks) > 0 {
		e.nextDPFnID++
		e.activeDPFnID = e.nextDPFnID
		e.activeDPParam0 = e.formatValue(fn.Params[0])
		if len(fn.Params) >= 2 {
			e.activeDPParam1 = e.formatValue(fn.Params[1])
		}
		buf.WriteString("dp_entry:\n")
		buf.WriteString("    %dp_out = alloca i64, align 8\n")
		buf.WriteString(fmt.Sprintf("    %%dp_hit = call i1 @joss_dp_get(i64 %d, i64 %s, i64 %s, i64* %%dp_out)\n",
			e.activeDPFnID, e.activeDPParam0, e.activeDPParam1))
		buf.WriteString(fmt.Sprintf("    br i1 %%dp_hit, label %%dp_ret, label %%%s\n", fn.Blocks[0].Label))
		buf.WriteString("dp_ret:\n")
		buf.WriteString("    %dp_val = load i64, i64* %dp_out, align 8\n")
		buf.WriteString("    ret i64 %dp_val\n")
	}

	for bi, block := range fn.Blocks {
		buf.WriteString(fmt.Sprintf("%s:\n", block.Label))
		if bi == 0 && fn.Name == "main" {
			if initFn := e.program.Functions["__joss_init_static_globals"]; initFn != nil {
				buf.WriteString(fmt.Sprintf("    call %s @__joss_init_static_globals()\n", MapLLVMType(initFn.ReturnType)))
			}
		}

		for _, inst := range block.Instructions {
			line, err := e.emitInstruction(inst)
			if err != nil {
				return err
			}
			if line != "" {
				buf.WriteString("    " + line + "\n")
			}
		}

		if block.Terminator != nil {
			termLine, err := e.emitTerminator(fn, block.Terminator)
			if err != nil {
				return err
			}
			buf.WriteString("    " + termLine + "\n")
		} else {
			return fmt.Errorf("llvm emitter: bloque %s en @%s no tiene terminador", block.Label, fn.Name)
		}
	}

	buf.WriteString("}\n")
	return nil
}

func (e *LLVMEmitter) emitInstruction(inst ir.Instruction) (string, error) {
	vol := ""
	if e.currentFnHasTry {
		vol = "volatile "
	}
	switch i := inst.(type) {
	case *ir.AllocaInst:
		allocType := MapLLVMType(i.AllocType)
		if i.Dest != nil {
			e.tempLLVMTypes[i.Dest.ID] = allocType + "*"
		}
		return fmt.Sprintf("%s = alloca %s, align 8", e.formatValue(i.Dest), allocType), nil

	case *ir.StoreInst:
		ptrType := MapLLVMType(i.Ptr.Type())
		if tv, ok := i.Ptr.(*ir.TempValue); ok {
			if known, exists := e.tempLLVMTypes[tv.ID]; exists {
				ptrType = known
			}
		}
		if strings.HasSuffix(ptrType, "*") {
			elemLLVM := strings.TrimSuffix(ptrType, "*")
			pre, valExpr := e.coerceLLVM(i.Val, elemLLVM, fmt.Sprintf("st_%p", i))
			return fmt.Sprintf("%sstore %s%s %s, %s %s, align 8", pre, vol, elemLLVM, valExpr, ptrType, e.formatValue(i.Ptr)), nil
		}
		valStr := e.formatTypedValue(i.Val)
		return fmt.Sprintf("store %s%s, %s %s, align 8", vol, valStr, ptrType, e.formatValue(i.Ptr)), nil

	case *ir.LoadInst:
		destType := MapLLVMType(i.Dest.ValueType)
		ptrType := MapLLVMType(i.Ptr.Type())
		if tv, ok := i.Ptr.(*ir.TempValue); ok {
			if known, exists := e.tempLLVMTypes[tv.ID]; exists {
				ptrType = known
				elem := strings.TrimSuffix(known, "*")
				if elem != destType {
					raw := fmt.Sprintf("%%ldraw_%p", i)
					conv := e.emitDirectCast(elem, raw, destType, e.formatValue(i.Dest))
					return fmt.Sprintf("%s = load %s%s, %s %s, align 8\n\t%s", raw, vol, elem, ptrType, e.formatValue(i.Ptr), conv), nil
				}
			}
		}
		return fmt.Sprintf("%s = load %s%s, %s %s, align 8", e.formatValue(i.Dest), vol, destType, ptrType, e.formatValue(i.Ptr)), nil

	case *ir.GlobalLoadInst:
		global := e.program.Globals[i.GlobalName]
		if global == nil {
			return "", fmt.Errorf("llvm emitter: global no registrada %q", i.GlobalName)
		}
		name := "@joss_global_" + cleanIdent(i.GlobalName)
		typeName := MapLLVMType(global.Type)
		destType := MapLLVMType(i.Dest.ValueType)
		if typeName != destType {
			raw := fmt.Sprintf("%%gld_%p", i)
			conv := e.emitDirectCast(typeName, raw, destType, e.formatValue(i.Dest))
			return fmt.Sprintf("%s = load %s, %s* %s, align 8\n\t%s", raw, typeName, typeName, name, conv), nil
		}
		return fmt.Sprintf("%s = load %s, %s* %s, align 8", e.formatValue(i.Dest), typeName, typeName, name), nil

	case *ir.GlobalStoreInst:
		global := e.program.Globals[i.GlobalName]
		if global == nil {
			return "", fmt.Errorf("llvm emitter: global no registrada %q", i.GlobalName)
		}
		if cleanIdent(i.GlobalName) == "__joss_request_params" {
			preReq, reqPtr := e.coerceLLVM(i.Val, "i8*", fmt.Sprintf("gst_req_%p", i))
			return fmt.Sprintf("%scall void @joss_set_active_request(i8* %s)", preReq, reqPtr), nil
		}
		name := "@joss_global_" + cleanIdent(i.GlobalName)
		typeName := MapLLVMType(global.Type)
		pre, valExpr := e.coerceLLVM(i.Val, typeName, fmt.Sprintf("gst_%p", i))
		return fmt.Sprintf("%sstore %s %s, %s* %s, align 8", pre, typeName, valExpr, typeName, name), nil

	case *ir.MoveInst:
		destType := MapLLVMType(i.Dest.ValueType)
		valStr := e.formatValue(i.Src)
		return fmt.Sprintf("%s = add %s 0, %s", e.formatValue(i.Dest), destType, valStr), nil

	case *ir.BinaryInst:
		llvmOp := llvmArithmeticOp(i.Op, i.Dest.ValueType)
		tStr := MapLLVMType(i.Dest.ValueType)
		if tStr == "i8*" {
			preL, lVal := e.coerceLLVM(i.Left, "i64", fmt.Sprintf("bl_%p", i))
			preR, rVal := e.coerceLLVM(i.Right, "i64", fmt.Sprintf("br_%p", i))
			raw := fmt.Sprintf("%%bin_i64_%p", i)
			return fmt.Sprintf("%s%s%s = %s i64 %s, %s\n\t%s = inttoptr i64 %s to i8*", preL, preR, raw, llvmOp, lVal, rVal, e.formatValue(i.Dest), raw), nil
		}
		preL, lVal := e.coerceLLVM(i.Left, tStr, fmt.Sprintf("bl_%p", i))
		preR, rVal := e.coerceLLVM(i.Right, tStr, fmt.Sprintf("br_%p", i))
		return fmt.Sprintf("%s%s%s = %s %s %s, %s", preL, preR, e.formatValue(i.Dest), llvmOp, tStr, lVal, rVal), nil

	case *ir.UnaryInst:
		if i.Op == ir.OpNot {
			pre, v := e.coerceLLVM(i.Val, "i1", fmt.Sprintf("un_%p", i))
			return fmt.Sprintf("%s%s = xor i1 %s, true", pre, e.formatValue(i.Dest), v), nil
		}
		if i.Op == ir.OpNeg {
			tStr := MapLLVMType(i.Dest.ValueType)
			pre, v := e.coerceLLVM(i.Val, tStr, fmt.Sprintf("ng_%p", i))
			if i.Dest.ValueType.IsFloat() {
				return fmt.Sprintf("%s%s = fsub %s 0.0, %s", pre, e.formatValue(i.Dest), tStr, v), nil
			}
			return fmt.Sprintf("%s%s = sub %s 0, %s", pre, e.formatValue(i.Dest), tStr, v), nil
		}

	case *ir.CompareInst:
		lT := MapLLVMType(i.Left.Type())
		rT := MapLLVMType(i.Right.Type())
		tStr := lT
		cmpType := i.Left.Type()
		if lT != rT {
			if lT == "double" || rT == "double" {
				tStr = "double"
				cmpType = ir.TypeF64
			} else if lT == "i64" || rT == "i64" {
				tStr = "i64"
				cmpType = ir.TypeI64
			} else if lT == "i8*" || rT == "i8*" {
				tStr = "i8*"
				cmpType = ir.TypeString
			}
		}
		preL, lVal := e.coerceLLVM(i.Left, tStr, fmt.Sprintf("cl_%p", i))
		preR, rVal := e.coerceLLVM(i.Right, tStr, fmt.Sprintf("cr_%p", i))
		if i.Op == ir.OpSpaceship {
			destVal := e.formatValue(i.Dest)
			ltTemp := destVal + "_lt"
			gtTemp := destVal + "_gt"
			selTemp := destVal + "_sel"
			return fmt.Sprintf("%s%s%s = icmp slt %s %s, %s\n\t%s = icmp sgt %s %s, %s\n\t%s = select i1 %s, i64 1, i64 0\n\t%s = select i1 %s, i64 -1, i64 %s",
				preL, preR,
				ltTemp, tStr, lVal, rVal,
				gtTemp, tStr, lVal, rVal,
				selTemp, gtTemp,
				destVal, ltTemp, selTemp), nil
		}
		if (i.Op == ir.OpCmpEq || i.Op == ir.OpCmpNe) && (i.Left.Type().Kind == ir.TypeKindString || i.Right.Type().Kind == ir.TypeKindString) {
			_, isLNull := i.Left.(*ir.ConstNull)
			_, isRNull := i.Right.(*ir.ConstNull)
			if !isLNull && !isRNull {
				preL, lVal := e.coerceLLVM(i.Left, "i8*", fmt.Sprintf("cl_%p", i))
				preR, rVal := e.coerceLLVM(i.Right, "i8*", fmt.Sprintf("cr_%p", i))
				if i.Op == ir.OpCmpEq {
					return fmt.Sprintf("%s%s%s = call i1 @joss_str_eq(i8* %s, i8* %s)", preL, preR, e.formatValue(i.Dest), lVal, rVal), nil
				}
				rawEq := fmt.Sprintf("%%streq_%p", i)
				return fmt.Sprintf("%s%s%s = call i1 @joss_str_eq(i8* %s, i8* %s)\n\t%s = xor i1 %s, true", preL, preR, rawEq, lVal, rVal, e.formatValue(i.Dest), rawEq), nil
			}
		}
		cmpOp := llvmCmpOp(i.Op, cmpType)
		return fmt.Sprintf("%s%s%s = %s %s %s, %s", preL, preR, e.formatValue(i.Dest), cmpOp, tStr, lVal, rVal), nil

	case *ir.CallInst:
		return e.emitDirectCall(i.Callee, i.Dest, i.Args, fmt.Sprintf("ci_%p", i)), nil

	case *ir.CallIndirectInst:
		args := make([]string, len(i.Args))
		argTypes := make([]string, len(i.Args))
		for idx, a := range i.Args {
			args[idx] = e.formatTypedValue(a)
			argTypes[idx] = MapLLVMType(a.Type())
		}
		argsJoined := strings.Join(args, ", ")
		preCallee, rawCallee := e.coerceLLVM(i.CalleeVal, "i8*", fmt.Sprintf("callee_%p", i))
		fnReady := fmt.Sprintf("%%fn_ready_%p", i)
		prepareLine := fmt.Sprintf("%s = call i8* @joss_closure_prepare_call(i8* %s)\n\t", fnReady, rawCallee)
		retType := "void"
		if i.Dest != nil && i.Dest.ValueType.Kind != ir.TypeKindVoid {
			retType = MapLLVMType(i.Dest.ValueType)
		}
		fnPtrType := fmt.Sprintf("%s (%s)*", retType, strings.Join(argTypes, ", "))
		fnTmp := fmt.Sprintf("%%fnptr_%p", i)
		castLine := fmt.Sprintf("%s = bitcast i8* %s to %s\n\t", fnTmp, fnReady, fnPtrType)
		if i.Dest != nil && i.Dest.ValueType.Kind != ir.TypeKindVoid {
			return fmt.Sprintf("%s%s%s%s = call %s %s(%s)", preCallee, prepareLine, castLine, e.formatValue(i.Dest), retType, fnTmp, argsJoined), nil
		}
		return fmt.Sprintf("%s%s%scall void %s(%s)", preCallee, prepareLine, castLine, fnTmp, argsJoined), nil

	case *ir.CallHostServiceInst:
		fnName := "fn_" + i.ClassName + "_" + i.MethodName
		args := i.Args
		if hasJossHostServiceImplementation(e.program, i.ClassName, i.MethodName) {
			fnName = jossHostServiceFunctionName(e.program, i.ClassName, i.MethodName)
		} else {
			args = normalizeLLVMHostServiceArgs(i.ClassName, i.MethodName, args)
		}
		return e.emitDirectCall(fnName, i.Dest, args, fmt.Sprintf("ch_%p", i)), nil

	case *ir.CallRuntimeInst:
		rtFunc := i.Func
		if !strings.HasPrefix(rtFunc, "joss_") {
			rtFunc = "joss_" + rtFunc
		}
		if (rtFunc == "joss_to_string" || rtFunc == "joss_strval") && len(i.Args) == 1 {
			if i.Args[0].Type().Kind == ir.TypeKindBool && i.Dest != nil {
				pre, bStr := e.coerceLLVM(i.Args[0], "i1", fmt.Sprintf("tsb_%p", i))
				tSym := e.registerString("true")
				fSym := e.registerString("false")
				tPtr := fmt.Sprintf("getelementptr inbounds ([5 x i8], [5 x i8]* %s, i64 0, i64 0)", tSym)
				fPtr := fmt.Sprintf("getelementptr inbounds ([6 x i8], [6 x i8]* %s, i64 0, i64 0)", fSym)
				return fmt.Sprintf("%s%s = select i1 %s, i8* %s, i8* %s", pre, e.formatValue(i.Dest), bStr, tPtr, fPtr), nil
			}
			if i.Args[0].Type().IsFloat() {
				return e.emitDirectCall("joss_f64_to_string", i.Dest, i.Args, fmt.Sprintf("tsf_%p", i)), nil
			}
			if i.Args[0].Type().IsInteger() {
				return e.emitDirectCall("joss_i64_to_str", i.Dest, i.Args, fmt.Sprintf("tsi_%p", i)), nil
			}
		}
		if rtFunc == "joss_buf_append_byte" && len(i.Args) == 2 {
			bTemp := fmt.Sprintf("%%bufapp_u8_%p", i)
			return fmt.Sprintf("%s = trunc i64 %s to i8\n\tcall void @joss_buf_append_byte(%s, i8 %s)",
				bTemp, e.formatValue(i.Args[1]), e.formatTypedValue(i.Args[0]), bTemp), nil
		}
		return e.emitDirectCall(rtFunc, i.Dest, i.Args, fmt.Sprintf("cr_%p", i)), nil

	case *ir.CallBuiltinInst:
		if e.program.Functions[i.BuiltinID] != nil {
			return e.emitDirectCall(i.BuiltinID, i.Dest, i.Args, fmt.Sprintf("cb_%p", i)), nil
		}
		if i.BuiltinID == "intval" && len(i.Args) == 1 && i.Dest != nil {
			if i.Args[0].Type().IsFloat() {
				pre, vStr := e.coerceLLVM(i.Args[0], "double", fmt.Sprintf("ivf_%p", i))
				return fmt.Sprintf("%s%s = fptosi double %s to i64", pre, e.formatValue(i.Dest), vStr), nil
			}
			if i.Args[0].Type().IsInteger() || i.Args[0].Type().Kind == ir.TypeKindBool {
				pre, vStr := e.coerceLLVM(i.Args[0], "i64", fmt.Sprintf("ivi_%p", i))
				return fmt.Sprintf("%s%s = add i64 %s, 0", pre, e.formatValue(i.Dest), vStr), nil
			}
		}
		if (i.BuiltinID == "floatval" || i.BuiltinID == "decimal") && len(i.Args) == 1 && i.Dest != nil {
			if i.Args[0].Type().IsInteger() {
				pre, vStr := e.coerceLLVM(i.Args[0], "i64", fmt.Sprintf("fvi_%p", i))
				return fmt.Sprintf("%s%s = sitofp i64 %s to double", pre, e.formatValue(i.Dest), vStr), nil
			}
			if i.Args[0].Type().IsFloat() {
				pre, vStr := e.coerceLLVM(i.Args[0], "double", fmt.Sprintf("fvf_%p", i))
				return fmt.Sprintf("%s%s = fadd double %s, 0.0", pre, e.formatValue(i.Dest), vStr), nil
			}
		}
		if (i.BuiltinID == "strval" || i.BuiltinID == "to_string") && len(i.Args) == 1 {
			if i.Args[0].Type().Kind == ir.TypeKindBool && i.Dest != nil {
				pre, bStr := e.coerceLLVM(i.Args[0], "i1", fmt.Sprintf("svb_%p", i))
				tSym := e.registerString("true")
				fSym := e.registerString("false")
				tPtr := fmt.Sprintf("getelementptr inbounds ([5 x i8], [5 x i8]* %s, i64 0, i64 0)", tSym)
				fPtr := fmt.Sprintf("getelementptr inbounds ([6 x i8], [6 x i8]* %s, i64 0, i64 0)", fSym)
				return fmt.Sprintf("%s%s = select i1 %s, i8* %s, i8* %s", pre, e.formatValue(i.Dest), bStr, tPtr, fPtr), nil
			}
			if i.Args[0].Type().IsFloat() {
				return e.emitDirectCall("joss_f64_to_string", i.Dest, i.Args, fmt.Sprintf("svf_%p", i)), nil
			}
			if i.Args[0].Type().IsInteger() {
				return e.emitDirectCall("joss_i64_to_str", i.Dest, i.Args, fmt.Sprintf("svi_%p", i)), nil
			}
		}
		args := normalizeLLVMBuiltinArgs(i.BuiltinID, i.Args)
		return e.emitDirectCall("joss_"+i.BuiltinID, i.Dest, args, fmt.Sprintf("cb_%p", i)), nil

	case *ir.CallPrimitiveMethodInst:
		return e.emitDirectCall(resolveLLVMPrimitiveMethodSym(i.TypeKind, i.MethodName), i.Dest, i.Args, fmt.Sprintf("cp_%p", i)), nil

	case *ir.NewObjectInst:
		clsSym := e.registerString(i.ClassName)
		escaped, length := formatLLVMStringLiteral(i.ClassName)
		_ = escaped
		destVal := e.formatValue(i.Dest)
		return fmt.Sprintf("%s = call i8* @joss_obj_new(i8* getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0))", destVal, length, length, clsSym), nil

	case *ir.LoadFieldInst:
		fieldSym := e.registerString(i.FieldName)
		_, length := formatLLVMStringLiteral(i.FieldName)
		destVal := e.formatValue(i.Dest)
		pre, objVal := e.coerceLLVM(i.Obj, "i8*", fmt.Sprintf("lf_%p", i))
		if i.Dest.Type().Kind == ir.TypeKindString {
			return fmt.Sprintf("%s%s = call i8* @joss_obj_get_field_str(i8* %s, i8* getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0))", pre, destVal, objVal, length, length, fieldSym), nil
		}
		destLLVM := MapLLVMType(i.Dest.Type())
		if destLLVM != "i64" {
			raw := fmt.Sprintf("%%lfraw_%p", i)
			conv := e.emitDirectCast("i64", raw, destLLVM, destVal)
			return fmt.Sprintf("%s%s = call i64 @joss_obj_get_field_i64(i8* %s, i8* getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0))\n\t%s", pre, raw, objVal, length, length, fieldSym, conv), nil
		}
		return fmt.Sprintf("%s%s = call i64 @joss_obj_get_field_i64(i8* %s, i8* getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0))", pre, destVal, objVal, length, length, fieldSym), nil

	case *ir.StoreFieldInst:
		fieldSym := e.registerString(i.FieldName)
		_, length := formatLLVMStringLiteral(i.FieldName)
		preObj, objVal := e.coerceLLVM(i.Obj, "i8*", fmt.Sprintf("sf_o_%p", i))
		if i.Val.Type().Kind == ir.TypeKindString {
			preVal, valVal := e.coerceLLVM(i.Val, "i8*", fmt.Sprintf("sf_v_%p", i))
			return fmt.Sprintf("%s%scall void @joss_obj_set_field_str(i8* %s, i8* getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0), i8* %s)", preObj, preVal, objVal, length, length, fieldSym, valVal), nil
		}
		preVal, valVal := e.coerceLLVM(i.Val, "i64", fmt.Sprintf("sf_v_%p", i))
		return fmt.Sprintf("%s%scall void @joss_obj_set_field_i64(i8* %s, i8* getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0), i64 %s)", preObj, preVal, objVal, length, length, fieldSym, valVal), nil

	case *ir.NewArrayInst:
		lines := []string{fmt.Sprintf("%s = call i8* @joss_arr_new(i64 %d)", e.formatValue(i.Dest), len(i.Elements))}
		for idx, el := range i.Elements {
			if el.Type().Kind == ir.TypeKindString {
				pre, ev := e.coerceLLVM(el, "i8*", fmt.Sprintf("na_%p_%d", i, idx))
				lines = append(lines, fmt.Sprintf("%scall void @joss_arr_push_str(i8* %s, i8* %s)", pre, e.formatValue(i.Dest), ev))
			} else {
				pre, ev := e.coerceLLVM(el, "i64", fmt.Sprintf("na_%p_%d", i, idx))
				lines = append(lines, fmt.Sprintf("%scall void @joss_arr_push_i64(i8* %s, i64 %s)", pre, e.formatValue(i.Dest), ev))
			}
		}
		return strings.Join(lines, "\n\t"), nil

	case *ir.ArrayGetInst:
		destVal := e.formatValue(i.Dest)
		preArr, arrVal := e.coerceLLVM(i.Array, "i8*", fmt.Sprintf("ag_a_%p", i))
		preIdx, idxVal := e.coerceLLVM(i.Index, "i64", fmt.Sprintf("ag_i_%p", i))
		if i.Dest.Type().Kind == ir.TypeKindString {
			return fmt.Sprintf("%s%s%s = call i8* @joss_arr_get_str(i8* %s, i64 %s)", preArr, preIdx, destVal, arrVal, idxVal), nil
		}
		destLLVM := MapLLVMType(i.Dest.Type())
		if destLLVM != "i64" {
			raw := fmt.Sprintf("%%agraw_%p", i)
			conv := e.emitDirectCast("i64", raw, destLLVM, destVal)
			return fmt.Sprintf("%s%s%s = call i64 @joss_arr_get_i64(i8* %s, i64 %s)\n\t%s", preArr, preIdx, raw, arrVal, idxVal, conv), nil
		}
		return fmt.Sprintf("%s%s%s = call i64 @joss_arr_get_i64(i8* %s, i64 %s)", preArr, preIdx, destVal, arrVal, idxVal), nil

	case *ir.ArraySetInst:
		preArr, arrVal := e.coerceLLVM(i.Array, "i8*", fmt.Sprintf("as_a_%p", i))
		preIdx, idxVal := e.coerceLLVM(i.Index, "i64", fmt.Sprintf("as_i_%p", i))
		if i.Val.Type().Kind == ir.TypeKindString {
			preVal, valVal := e.coerceLLVM(i.Val, "i8*", fmt.Sprintf("as_v_%p", i))
			return fmt.Sprintf("%s%s%scall void @joss_arr_set_str(i8* %s, i64 %s, i8* %s)", preArr, preIdx, preVal, arrVal, idxVal, valVal), nil
		}
		preVal, valVal := e.coerceLLVM(i.Val, "i64", fmt.Sprintf("as_v_%p", i))
		return fmt.Sprintf("%s%s%scall void @joss_arr_set_i64(i8* %s, i64 %s, i64 %s)", preArr, preIdx, preVal, arrVal, idxVal, valVal), nil

	case *ir.NewMapInst:
		lines := []string{fmt.Sprintf("%s = call i8* @joss_map_new()", e.formatValue(i.Dest))}
		for idx, p := range i.Pairs {
			preK, keyVal := e.coerceLLVM(p[0], "i8*", fmt.Sprintf("nm_k_%p_%d", i, idx))
			if p[1].Type().Kind == ir.TypeKindString {
				preV, valVal := e.coerceLLVM(p[1], "i8*", fmt.Sprintf("nm_v_%p_%d", i, idx))
				lines = append(lines, fmt.Sprintf("%s%scall void @joss_map_set_str(i8* %s, i8* %s, i8* %s)", preK, preV, e.formatValue(i.Dest), keyVal, valVal))
			} else {
				preV, valVal := e.coerceLLVM(p[1], "i64", fmt.Sprintf("nm_v_%p_%d", i, idx))
				lines = append(lines, fmt.Sprintf("%s%scall void @joss_map_set_i64(i8* %s, i8* %s, i64 %s)", preK, preV, e.formatValue(i.Dest), keyVal, valVal))
			}
		}
		return strings.Join(lines, "\n\t"), nil

	case *ir.MapGetInst:
		destVal := e.formatValue(i.Dest)
		preM, mapVal := e.coerceLLVM(i.Map, "i8*", fmt.Sprintf("mg_m_%p", i))
		preK, keyVal := e.coerceLLVM(i.Key, "i8*", fmt.Sprintf("mg_k_%p", i))
		if i.Dest.Type().Kind == ir.TypeKindString {
			return fmt.Sprintf("%s%s%s = call i8* @joss_map_get_str(i8* %s, i8* %s)", preM, preK, destVal, mapVal, keyVal), nil
		}
		if i.Dest.Type().Kind == ir.TypeKindBool {
			raw := fmt.Sprintf("%%mgraw_%p", i)
			return fmt.Sprintf("%s%s%s = call i64 @joss_map_get_i64(i8* %s, i8* %s)\n\t%s = call i1 @joss_is_truthy(i64 %s)", preM, preK, raw, mapVal, keyVal, destVal, raw), nil
		}
		destLLVM := MapLLVMType(i.Dest.Type())
		if destLLVM != "i64" {
			raw := fmt.Sprintf("%%mgraw_%p", i)
			conv := e.emitDirectCast("i64", raw, destLLVM, destVal)
			return fmt.Sprintf("%s%s%s = call i64 @joss_map_get_i64(i8* %s, i8* %s)\n\t%s", preM, preK, raw, mapVal, keyVal, conv), nil
		}
		if i.Dest.Type().Kind == ir.TypeKindI64 {
			raw := fmt.Sprintf("%%mgraw_%p", i)
			return fmt.Sprintf("%s%s%s = call i64 @joss_map_get_i64(i8* %s, i8* %s)\n\t%s = call i64 @joss_unbox_value(i64 %s)", preM, preK, raw, mapVal, keyVal, destVal, raw), nil
		}
		return fmt.Sprintf("%s%s%s = call i64 @joss_map_get_i64(i8* %s, i8* %s)", preM, preK, destVal, mapVal, keyVal), nil

	case *ir.MapSetInst:
		preM, mapVal := e.coerceLLVM(i.Map, "i8*", fmt.Sprintf("ms_m_%p", i))
		preK, keyVal := e.coerceLLVM(i.Key, "i8*", fmt.Sprintf("ms_k_%p", i))
		if i.Val.Type().Kind == ir.TypeKindString {
			preV, valVal := e.coerceLLVM(i.Val, "i8*", fmt.Sprintf("ms_v_%p", i))
			return fmt.Sprintf("%s%s%scall void @joss_map_set_str(i8* %s, i8* %s, i8* %s)", preM, preK, preV, mapVal, keyVal, valVal), nil
		}
		preV, valVal := e.coerceLLVM(i.Val, "i64", fmt.Sprintf("ms_v_%p", i))
		return fmt.Sprintf("%s%s%scall void @joss_map_set_i64(i8* %s, i8* %s, i64 %s)", preM, preK, preV, mapVal, keyVal, valVal), nil

	case *ir.MapDeleteInst:
		preM, mapVal := e.coerceLLVM(i.Map, "i8*", fmt.Sprintf("md_m_%p", i))
		preK, keyVal := e.coerceLLVM(i.Key, "i8*", fmt.Sprintf("md_k_%p", i))
		return fmt.Sprintf("%s%scall void @joss_map_delete_i64(i8* %s, i8* %s)", preM, preK, mapVal, keyVal), nil

	case *ir.BoxValueInst:
		var tag int64 = 1
		switch i.Val.Type().Kind {
		case ir.TypeKindBool:
			tag = 3
		case ir.TypeKindF64, ir.TypeKindF32, ir.TypeKindDecimal:
			tag = 2
		case ir.TypeKindString:
			tag = 4
		case ir.TypeKindPtr:
			if i.Val.Type().Elem != nil && i.Val.Type().Elem.Kind == ir.TypeKindVoid {
				tag = 0
			} else {
				tag = 5
			}
		}
		destVal := e.formatValue(i.Dest)
		valStr := e.formatValue(i.Val)
		payloadExpr := valStr
		convLine := ""
		switch i.Val.Type().Kind {
		case ir.TypeKindBool:
			payloadExpr = destVal + "_zext"
			convLine = fmt.Sprintf("%s = zext i1 %s to i64\n\t", payloadExpr, valStr)
		case ir.TypeKindF64, ir.TypeKindDecimal:
			payloadExpr = destVal + "_bits"
			convLine = fmt.Sprintf("%s = bitcast double %s to i64\n\t", payloadExpr, valStr)
		case ir.TypeKindString, ir.TypeKindPtr, ir.TypeKindStruct, ir.TypeKindArray:
			payloadExpr = destVal + "_p2i"
			convLine = fmt.Sprintf("%s = ptrtoint %s %s to i64\n\t", payloadExpr, MapLLVMType(i.Val.Type()), valStr)
		}
		boxPtr := destVal + "_boxptr"
		return fmt.Sprintf("%s%s = call i8* @joss_box_value(i64 %d, i64 %s)\n\t%s = ptrtoint i8* %s to i64",
			convLine, boxPtr, tag, payloadExpr, destVal, boxPtr), nil

	case *ir.UnboxValueInst:
		pre, v := e.coerceLLVM(i.Val, "i64", fmt.Sprintf("ub_%p", i))
		destType := MapLLVMType(i.Dest.ValueType)
		if destType != "i64" {
			raw := fmt.Sprintf("%%ubraw_%p", i)
			conv := e.emitDirectCast("i64", raw, destType, e.formatValue(i.Dest))
			return fmt.Sprintf("%s%s = call i64 @joss_unbox_value(i64 %s)\n\t%s", pre, raw, v, conv), nil
		}
		return fmt.Sprintf("%s%s = call i64 @joss_unbox_value(i64 %s)", pre, e.formatValue(i.Dest), v), nil

	case *ir.AllocBufferInst:
		return fmt.Sprintf("%s = call i8* @joss_buf_new(i64 %s)", e.formatValue(i.Dest), e.formatValue(i.Cap)), nil

	case *ir.FreeBufferInst:
		return fmt.Sprintf("call void @joss_buf_free(i8* %s)", e.formatValue(i.Buf)), nil

	case *ir.BufferLenInst:
		return fmt.Sprintf("%s = call i64 @joss_buf_len(i8* %s)", e.formatValue(i.Dest), e.formatValue(i.Buf)), nil

	case *ir.BufferGetInst:
		destVal := e.formatValue(i.Dest)
		bTemp := destVal + "_u8"
		return fmt.Sprintf("%s = call i8 @joss_buf_get(i8* %s, i64 %s)\n\t%s = zext i8 %s to i64",
			bTemp, e.formatValue(i.Buf), e.formatValue(i.Index), destVal, bTemp), nil

	case *ir.BufferSetInst:
		bTemp := fmt.Sprintf("%%bufset_u8_%p", i)
		return fmt.Sprintf("%s = trunc i64 %s to i8\n\tcall void @joss_buf_set(i8* %s, i64 %s, i8 %s)",
			bTemp, e.formatValue(i.Val), e.formatValue(i.Buf), e.formatValue(i.Index), bTemp), nil

	case *ir.ExternCallInst:
		args := make([]string, len(i.Args))
		for idx, a := range i.Args {
			args[idx] = e.formatTypedValue(a)
		}
		argsJoined := strings.Join(args, ", ")
		if i.Dest != nil && i.RetType.Kind != ir.TypeKindVoid {
			return fmt.Sprintf("%s = call %s @%s(%s)", e.formatValue(i.Dest), MapLLVMType(i.RetType), i.Symbol, argsJoined), nil
		}
		return fmt.Sprintf("call void @%s(%s)", i.Symbol, argsJoined), nil

	case *ir.ThrowInst:
		if i.Val != nil && i.Val.Type().Kind == ir.TypeKindString {
			return fmt.Sprintf("call void @joss_throw(%s)", e.formatTypedValue(i.Val)), nil
		}
		if i.Val != nil {
			pre, vStr := e.coerceLLVM(i.Val, "i8*", fmt.Sprintf("thr_%p", i))
			return fmt.Sprintf("%scall void @joss_throw(i8* %s)", pre, vStr), nil
		}
		msgSym := e.registerString("Joss Exception")
		_, msgLen := formatLLVMStringLiteral("Joss Exception")
		return fmt.Sprintf("call void @joss_throw(i8* getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0))", msgLen, msgLen, msgSym), nil

	case *ir.CastInst:
		srcT := i.Val.Type()
		dstT := i.TargetType
		destVal := e.formatValue(i.Dest)
		srcVal := e.formatValue(i.Val)
		srcLLVM := MapLLVMType(srcT)
		dstLLVM := MapLLVMType(dstT)
		if srcLLVM == dstLLVM {
			if dstLLVM == "i8*" {
				return fmt.Sprintf("%s = bitcast i8* %s to i8*", destVal, srcVal), nil
			}
			if dstT.IsFloat() {
				return fmt.Sprintf("%s = fadd %s 0.0, %s", destVal, dstLLVM, srcVal), nil
			}
			return fmt.Sprintf("%s = add %s 0, %s", destVal, dstLLVM, srcVal), nil
		}
		if srcT.IsInteger() && dstT.IsFloat() {
			return fmt.Sprintf("%s = sitofp %s %s to %s", destVal, srcLLVM, srcVal, dstLLVM), nil
		}
		if srcT.IsFloat() && dstT.IsInteger() {
			return fmt.Sprintf("%s = fptosi %s %s to %s", destVal, srcLLVM, srcVal, dstLLVM), nil
		}
		if srcLLVM == "i64" && dstLLVM == "i8*" {
			return fmt.Sprintf("%s = inttoptr i64 %s to i8*", destVal, srcVal), nil
		}
		if srcLLVM == "i8*" && dstLLVM == "i64" {
			return fmt.Sprintf("%s = ptrtoint i8* %s to i64", destVal, srcVal), nil
		}
		if srcLLVM == "i1" && dstLLVM == "i64" {
			return fmt.Sprintf("%s = zext i1 %s to i64", destVal, srcVal), nil
		}
		if srcLLVM == "i64" && dstLLVM == "i1" {
			return fmt.Sprintf("%s = icmp ne i64 %s, 0", destVal, srcVal), nil
		}
		return fmt.Sprintf("%s = bitcast %s %s to %s", destVal, srcLLVM, srcVal, dstLLVM), nil

	case *ir.TryCatchInst:
		id := fmt.Sprintf("%p", i)
		tryLbl := "joss_try_" + id
		catchLbl := "joss_catch_" + id
		endLbl := "joss_try_end_" + id
		var collectTargets func([]*ir.BasicBlock, map[string]bool)
		collectTargets = func(blocks []*ir.BasicBlock, out map[string]bool) {
			for _, b := range blocks {
				for _, sub := range b.Instructions {
					if nested, ok := sub.(*ir.TryCatchInst); ok {
						collectTargets(nested.TryBlocks, out)
						collectTargets(nested.CatchBlocks, out)
					}
				}
				if b.Terminator != nil {
					for _, succ := range b.Terminator.Successors() {
						if succ != nil {
							out[succ.Label] = true
						}
					}
				}
			}
		}
		tryTargets := make(map[string]bool)
		collectTargets(i.TryBlocks, tryTargets)
		catchTargets := make(map[string]bool)
		collectTargets(i.CatchBlocks, catchTargets)

		var lines []string
		lines = append(lines, fmt.Sprintf("%%jb_%s = call i8* @joss_try_push()", id))
		if e.TargetOS == "windows" {
			sjSym := "@setjmp"
			if e.TargetArch == "arm64" {
				sjSym = "@__intrinsic_setjmpex"
			}
			lines = append(lines, fmt.Sprintf("%%sj_%s = call i32 %s(i8* %%jb_%s, i8* null) returns_twice", id, sjSym, id))
		} else {
			lines = append(lines, fmt.Sprintf("%%sj_%s = call i32 @setjmp(i8* %%jb_%s) returns_twice", id, id))
		}
		lines = append(lines, fmt.Sprintf("%%isc_%s = icmp ne i32 %%sj_%s, 0", id, id))
		lines = append(lines, fmt.Sprintf("br i1 %%isc_%s, label %%%s, label %%%s", id, catchLbl, tryLbl))
		lines = append(lines, tryLbl+":")
		if len(i.TryBlocks) > 0 && tryTargets[i.TryBlocks[0].Label] {
			lines = append(lines, "\tbr label %"+i.TryBlocks[0].Label)
		}
		lastTryHasTerm := false
		for bi, tb := range i.TryBlocks {
			if bi > 0 {
				if lastTryHasTerm && !tryTargets[tb.Label] {
					continue
				}
				if !lastTryHasTerm {
					lines = append(lines, "\tbr label %"+tb.Label)
				}
				lines = append(lines, tb.Label+":")
			} else if tryTargets[tb.Label] {
				lines = append(lines, tb.Label+":")
			}
			lastTryHasTerm = false
			for _, sub := range tb.Instructions {
				s, err := e.emitInstruction(sub)
				if err != nil {
					return "", err
				}
				if s != "" {
					lines = append(lines, "\t"+s)
				}
			}
			if tb.Terminator != nil {
				if _, isRet := tb.Terminator.(*ir.ReturnTerminator); isRet {
					lines = append(lines, "\tcall void @joss_try_pop()")
				}
				ts, err := e.emitTerminator(e.currentFn, tb.Terminator)
				if err != nil {
					return "", err
				}
				lines = append(lines, "\t"+ts)
				lastTryHasTerm = true
			}
		}
		if !lastTryHasTerm {
			lines = append(lines, "\tcall void @joss_try_pop()")
			lines = append(lines, "\tbr label %"+endLbl)
		}
		lines = append(lines, catchLbl+":")
		if i.CatchVarPtr != nil {
			lines = append(lines, fmt.Sprintf("\t%%cmsg_%s = call i8* @joss_catch_msg()", id))
			ptrStr := e.formatValue(i.CatchVarPtr)
			if i.CatchVarPtr.Type().Elem != nil && i.CatchVarPtr.Type().Elem.Kind == ir.TypeKindString {
				lines = append(lines, fmt.Sprintf("\tstore i8* %%cmsg_%s, i8** %s, align 8", id, ptrStr))
			} else {
				lines = append(lines, fmt.Sprintf("\t%%cmsg_i64_%s = ptrtoint i8* %%cmsg_%s to i64", id, id))
				lines = append(lines, fmt.Sprintf("\tstore i64 %%cmsg_i64_%s, i64* %s, align 8", id, ptrStr))
			}
		}
		if len(i.CatchBlocks) > 0 && catchTargets[i.CatchBlocks[0].Label] {
			lines = append(lines, "\tbr label %"+i.CatchBlocks[0].Label)
		}
		lastCatchHasTerm := false
		for bi, cb := range i.CatchBlocks {
			if bi > 0 {
				if lastCatchHasTerm && !catchTargets[cb.Label] {
					continue
				}
				if !lastCatchHasTerm {
					lines = append(lines, "\tbr label %"+cb.Label)
				}
				lines = append(lines, cb.Label+":")
			} else if catchTargets[cb.Label] {
				lines = append(lines, cb.Label+":")
			}
			lastCatchHasTerm = false
			for _, sub := range cb.Instructions {
				s, err := e.emitInstruction(sub)
				if err != nil {
					return "", err
				}
				if s != "" {
					lines = append(lines, "\t"+s)
				}
			}
			if cb.Terminator != nil {
				ts, err := e.emitTerminator(e.currentFn, cb.Terminator)
				if err != nil {
					return "", err
				}
				lines = append(lines, "\t"+ts)
				lastCatchHasTerm = true
			}
		}
		if !lastCatchHasTerm {
			lines = append(lines, "\tbr label %"+endLbl)
		}
		lines = append(lines, endLbl+":")
		return strings.Join(lines, "\n"), nil
	}

	return "", fmt.Errorf("llvm emitter: instrucción no soportada %s", inst)
}

func (e *LLVMEmitter) emitTerminator(fn *ir.Function, term ir.Terminator) (string, error) {
	switch t := term.(type) {
	case *ir.ReturnTerminator:
		if t.Val == nil || fn.ReturnType.Kind == ir.TypeKindVoid {
			return "ret void", nil
		}
		retLLVM := MapLLVMType(fn.ReturnType)
		pre, valExpr := e.coerceLLVM(t.Val, retLLVM, fmt.Sprintf("ret_%p", t))
		if e.activeDPFnID > 0 && retLLVM == "i64" {
			return fmt.Sprintf("%scall void @joss_dp_put(i64 %d, i64 %s, i64 %s, i64 %s)\n\tret i64 %s",
				pre, e.activeDPFnID, e.activeDPParam0, e.activeDPParam1, valExpr, valExpr), nil
		}
		return fmt.Sprintf("%sret %s %s", pre, retLLVM, valExpr), nil

	case *ir.JumpTerminator:
		return fmt.Sprintf("br label %%%s", t.Target.Label), nil

	case *ir.BranchTerminator:
		pre, condExpr := e.coerceLLVM(t.Cond, "i1", fmt.Sprintf("br_%p", t))
		return fmt.Sprintf("%sbr i1 %s, label %%%s, label %%%s", pre, condExpr, t.TrueBlock.Label, t.FalseBlock.Label), nil

	case *ir.UnreachableTerminator:
		return "unreachable", nil
	}

	return "", fmt.Errorf("llvm emitter: terminador no soportado %s", term)
}

func (e *LLVMEmitter) formatTypedValue(val ir.Value) string {
	if cs, ok := val.(*ir.ConstString); ok {
		sym := e.registerString(cs.Val)
		_, length := formatLLVMStringLiteral(cs.Val)
		return fmt.Sprintf("i8* getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0)", length, length, sym)
	}
	return fmt.Sprintf("%s %s", MapLLVMType(val.Type()), e.formatValue(val))
}

func (e *LLVMEmitter) formatValue(val ir.Value) string {
	switch v := val.(type) {
	case *ir.TempValue:
		return fmt.Sprintf("%%t%d", v.ID)
	case *ir.ParamValue:
		if v.Name != "" {
			return "%" + cleanIdent(v.Name)
		}
		return fmt.Sprintf("%%arg%d", v.Index)
	case *ir.ConstInt:
		if v.ValueType.IsFloat() {
			return fmt.Sprintf("%d.0", v.Val)
		}
		return strconv.FormatInt(v.Val, 10)
	case *ir.ConstFloat:
		s := strconv.FormatFloat(v.Val, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s
	case *ir.ConstBool:
		if v.Val {
			return "true"
		}
		return "false"
	case *ir.ConstString:
		sym := e.registerString(v.Val)
		_, length := formatLLVMStringLiteral(v.Val)
		return fmt.Sprintf("getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0)", length, length, sym)
	case *ir.ConstNull:
		switch MapLLVMType(v.ValueType) {
		case "i8*":
			return "null"
		case "double":
			return "0.0"
		case "i1":
			return "false"
		default:
			return "0"
		}
	case *ir.GlobalValue:
		if e.program != nil {
			if _, ok := e.program.Globals[v.Name]; ok {
				return "@" + v.Name
			}
			if fn, ok := e.program.Functions[v.Name]; ok && fn != nil {
				retT := MapLLVMType(fn.ReturnType)
				paramTypes := make([]string, len(fn.Params))
				for idx, p := range fn.Params {
					paramTypes[idx] = MapLLVMType(p.ValueType)
				}
				return fmt.Sprintf("bitcast (%s (%s)* @%s to i8*)", retT, strings.Join(paramTypes, ", "), fn.Name)
			}
		}
		return "null"
	case *ir.ConstFnPtr:
		if e.program != nil {
			if fn, ok := e.program.Functions[v.FnName]; ok && fn != nil {
				retT := MapLLVMType(fn.ReturnType)
				paramTypes := make([]string, len(fn.Params))
				for idx, p := range fn.Params {
					paramTypes[idx] = MapLLVMType(p.ValueType)
				}
				return fmt.Sprintf("bitcast (%s (%s)* @%s to i8*)", retT, strings.Join(paramTypes, ", "), fn.Name)
			}
		}
		return "null"
	default:
		return val.String()
	}
}

func llvmArithmeticOp(op ir.Opcode, t ir.Type) string {
	isFloat := t.IsFloat()
	switch op {
	case ir.OpAdd:
		if isFloat {
			return "fadd"
		}
		return "add"
	case ir.OpSub:
		if isFloat {
			return "fsub"
		}
		return "sub"
	case ir.OpMul:
		if isFloat {
			return "fmul"
		}
		return "mul"
	case ir.OpDiv:
		if isFloat {
			return "fdiv"
		}
		return "sdiv"
	case ir.OpMod:
		if isFloat {
			return "frem"
		}
		return "srem"
	case ir.OpShl:
		return "shl"
	case ir.OpShr:
		return "ashr"
	case ir.OpAnd:
		return "and"
	case ir.OpOr:
		return "or"
	case ir.OpXor:
		return "xor"
	default:
		return "add"
	}
}

func llvmCmpOp(op ir.Opcode, t ir.Type) string {
	isFloat := t.IsFloat()
	if isFloat {
		switch op {
		case ir.OpCmpEq:
			return "fcmp oeq"
		case ir.OpCmpNe:
			return "fcmp one"
		case ir.OpCmpLt:
			return "fcmp olt"
		case ir.OpCmpLe:
			return "fcmp ole"
		case ir.OpCmpGt:
			return "fcmp ogt"
		case ir.OpCmpGe:
			return "fcmp oge"
		}
	}
	switch op {
	case ir.OpCmpEq:
		return "icmp eq"
	case ir.OpCmpNe:
		return "icmp ne"
	case ir.OpCmpLt:
		return "icmp slt"
	case ir.OpCmpLe:
		return "icmp sle"
	case ir.OpCmpGt:
		return "icmp sgt"
	case ir.OpCmpGe:
		return "icmp sge"
	default:
		return "icmp eq"
	}
}

func cleanIdent(name string) string {
	s := strings.TrimPrefix(name, "$")
	s = strings.ReplaceAll(s, "::", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, "<", "_")
	s = strings.ReplaceAll(s, ">", "_")
	s = strings.ReplaceAll(s, ",", "_")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func resolveLLVMPrimitiveMethodSym(typeKind, methodName string) string {
	tk := strings.ToLower(typeKind)
	switch {
	case strings.HasPrefix(tk, "array"):
		switch methodName {
		case "length", "count":
			return "joss_arr_len"
		case "push":
			return "joss_arr_push_ret"
		case "pop":
			return "joss_array_pop"
		}
	case strings.HasPrefix(tk, "string"):
		if methodName == "length" {
			return "joss_strlen"
		}
	case strings.HasPrefix(tk, "map"):
		if methodName == "length" || methodName == "count" {
			return "joss_map_len"
		}
	}
	return fmt.Sprintf("joss_%s_%s", cleanIdent(typeKind), cleanIdent(methodName))
}

func normalizePrimitiveMethodArgs(methodName string, args []ir.Value) []ir.Value {
	switch methodName {
	case "length", "count", "pop":
		if len(args) > 1 {
			return args[:1]
		}
	case "push":
		if len(args) > 2 {
			return []ir.Value{args[0], args[len(args)-1]}
		}
	}
	return args
}

func normalizeLLVMBuiltinArgs(builtinID string, args []ir.Value) []ir.Value {
	switch builtinID {
	case "rand":
		if len(args) == 0 {
			return []ir.Value{ir.NewConstInt(0, ir.TypeI64), ir.NewConstInt(2147483647, ir.TypeI64)}
		}
		if len(args) == 1 {
			return []ir.Value{ir.NewConstInt(0, ir.TypeI64), args[0]}
		}
	case "substr":
		if len(args) == 2 {
			return append([]ir.Value{args[0], args[1], ir.NewConstInt(-9223372036854775808, ir.TypeI64)}, args[2:]...)
		}
	case "env":
		if len(args) == 1 {
			return []ir.Value{args[0], ir.NewConstString("")}
		}
	case "request", "session", "gmdate", "date", "totp_generate":
		if len(args) == 1 {
			return []ir.Value{args[0], ir.NewConstInt(0, ir.TypeI64)}
		}
	case "totp_verify":
		if len(args) == 2 {
			return []ir.Value{args[0], args[1], ir.NewConstInt(0, ir.TypeI64)}
		}
	case "json":
		if len(args) == 1 {
			return []ir.Value{args[0], ir.NewConstInt(200, ir.TypeI64)}
		}
	case "redirect":
		if len(args) == 1 {
			return []ir.Value{args[0], ir.NewConstInt(302, ir.TypeI64)}
		}
	case "view":
		if len(args) == 1 {
			return []ir.Value{args[0], ir.NewConstNull(ir.PtrType(ir.TypeVoid))}
		}
	case "response":
		if len(args) == 1 {
			return []ir.Value{args[0], ir.NewConstInt(200, ir.TypeI64), ir.NewConstString("text/html; charset=utf-8")}
		}
		if len(args) == 2 {
			return []ir.Value{args[0], args[1], ir.NewConstString("text/html; charset=utf-8")}
		}
	}
	return args
}

func normalizeLLVMHostServiceArgs(className, methodName string, args []ir.Value) []ir.Value {
	switch className {
	case "Response":
		switch methodName {
		case "json":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstInt(200, ir.TypeI64)}
			}
		case "raw":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstInt(200, ir.TypeI64), ir.NewConstString("text/html; charset=utf-8")}
			}
			if len(args) == 2 {
				return []ir.Value{args[0], args[1], ir.NewConstString("text/html; charset=utf-8")}
			}
		case "error":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstInt(500, ir.TypeI64)}
			}
		case "download":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstString("")}
			}
		case "stream":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstString("application/octet-stream")}
			}
		case "redirect":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstInt(302, ir.TypeI64)}
			}
		}
	case "Plugin":
		switch methodName {
		case "call", "stream":
			if len(args) == 2 {
				return []ir.Value{args[0], args[1], ir.NewConstNull(ir.PtrType(ir.TypeVoid))}
			}
		case "path":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstString("")}
			}
		}
	case "System":
		switch methodName {
		case "Run":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstNull(ir.PtrType(ir.TypeVoid))}
			}
		case "env":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstInt(0, ir.TypeI64)}
			}
		case "now":
			if len(args) == 0 {
				return []ir.Value{ir.NewConstInt(0, ir.TypeI64)}
			}
		}
	case "Process":
		if methodName == "constructor" && len(args) == 2 {
			return []ir.Value{args[0], args[1], ir.NewConstNull(ir.PtrType(ir.TypeVoid))}
		}
	case "Sitemap":
		if methodName == "add" {
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstNull(ir.TypeString), ir.NewConstNull(ir.TypeString)}
			}
			if len(args) == 2 {
				return []ir.Value{args[0], args[1], ir.NewConstNull(ir.TypeString)}
			}
		}
	case "Http":
		switch methodName {
		case "get":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstNull(ir.PtrType(ir.TypeVoid))}
			}
		case "post", "json":
			if len(args) == 1 {
				return []ir.Value{args[0], ir.NewConstString(""), ir.NewConstNull(ir.PtrType(ir.TypeVoid))}
			}
			if len(args) == 2 {
				return []ir.Value{args[0], args[1], ir.NewConstNull(ir.PtrType(ir.TypeVoid))}
			}
		case "request":
			if len(args) == 2 {
				return []ir.Value{args[0], args[1], ir.NewConstString(""), ir.NewConstNull(ir.PtrType(ir.TypeVoid))}
			}
			if len(args) == 3 {
				return []ir.Value{args[0], args[1], args[2], ir.NewConstNull(ir.PtrType(ir.TypeVoid))}
			}
		}
	case "Cache":
		if methodName == "get" && len(args) == 1 {
			return []ir.Value{args[0], ir.NewConstInt(0, ir.TypeI64)}
		}
	}
	return args
}

func (e *LLVMEmitter) coerceLLVM(val ir.Value, targetLLVM string, tag string) (string, string) {
	if val == nil {
		if targetLLVM == "i8*" {
			return "", "null"
		}
		if targetLLVM == "double" {
			return "", "0.0"
		}
		if targetLLVM == "i1" {
			return "", "false"
		}
		return "", "0"
	}
	if _, ok := val.(*ir.ConstNull); ok {
		switch targetLLVM {
		case "i8*":
			return "", "null"
		case "double":
			return "", "0.0"
		case "i1":
			return "", "false"
		default:
			return "", "0"
		}
	}
	if cfp, ok := val.(*ir.ConstFnPtr); ok {
		fn := e.program.Functions[cfp.FnName]
		if fn == nil {
			if targetLLVM == "i64" {
				return "", "0"
			}
			return "", "null"
		}
		pTs := make([]string, len(fn.Params))
		for idx, p := range fn.Params {
			pTs[idx] = MapLLVMType(p.Type())
		}
		fnSig := fmt.Sprintf("%s (%s)*", MapLLVMType(fn.ReturnType), strings.Join(pTs, ", "))
		tmp := fmt.Sprintf("%%coerce_%s", tag)
		if targetLLVM == "i64" {
			return fmt.Sprintf("%s = ptrtoint %s @%s to i64\n\t", tmp, fnSig, cfp.FnName), tmp
		}
		return fmt.Sprintf("%s = bitcast %s @%s to %s\n\t", tmp, fnSig, cfp.FnName, targetLLVM), tmp
	}
	if ci, ok := val.(*ir.ConstInt); ok && targetLLVM == "double" {
		return "", fmt.Sprintf("%d.0", ci.Val)
	}
	srcLLVM := MapLLVMType(val.Type())
	valStr := e.formatValue(val)
	if srcLLVM == targetLLVM {
		return "", valStr
	}
	tmp := fmt.Sprintf("%%coerce_%s", tag)
	switch {
	case srcLLVM == "i64" && targetLLVM == "i8*":
		return fmt.Sprintf("%s = inttoptr i64 %s to i8*\n\t", tmp, valStr), tmp
	case srcLLVM == "i8*" && targetLLVM == "i64":
		return fmt.Sprintf("%s = ptrtoint i8* %s to i64\n\t", tmp, valStr), tmp
	case srcLLVM == "i1" && targetLLVM == "i64":
		return fmt.Sprintf("%s = zext i1 %s to i64\n\t", tmp, valStr), tmp
	case srcLLVM == "i64" && targetLLVM == "i1":
		return fmt.Sprintf("%s = icmp ne i64 %s, 0\n\t", tmp, valStr), tmp
	case srcLLVM == "double" && targetLLVM == "i64":
		return fmt.Sprintf("%s = bitcast double %s to i64\n\t", tmp, valStr), tmp
	case srcLLVM == "i64" && targetLLVM == "double":
		return fmt.Sprintf("%s = bitcast i64 %s to double\n\t", tmp, valStr), tmp
	case srcLLVM == "i8*" && targetLLVM == "i1":
		return fmt.Sprintf("%s = icmp ne i8* %s, null\n\t", tmp, valStr), tmp
	case srcLLVM == "i1" && targetLLVM == "i8*":
		return fmt.Sprintf("%s_z = zext i1 %s to i64\n\t%s = inttoptr i64 %s_z to i8*\n\t", tmp, valStr, tmp, tmp), tmp
	case srcLLVM == "float" && targetLLVM == "double":
		return fmt.Sprintf("%s = fpext float %s to double\n\t", tmp, valStr), tmp
	case (srcLLVM == "i8" || srcLLVM == "i16" || srcLLVM == "i32") && targetLLVM == "i64":
		extOp := "sext"
		if val.Type().Kind == ir.TypeKindU8 || val.Type().Kind == ir.TypeKindU16 || val.Type().Kind == ir.TypeKindU32 {
			extOp = "zext"
		}
		return fmt.Sprintf("%s = %s %s %s to i64\n\t", tmp, extOp, srcLLVM, valStr), tmp
	}
	return "", valStr
}

func (e *LLVMEmitter) emitDirectCall(sym string, dest *ir.TempValue, args []ir.Value, tag string) string {
	var pre strings.Builder
	if sym == "substr" {
		sym = "joss_substr"
	}
	userFn := e.program.Functions[sym]
	if userFn == nil && (strings.HasSuffix(sym, "_Init") || sym == "whereLike" || sym == "orWhereLike" || sym == "call" || sym == "tokens" || sym == "data") {
		if dest == nil || dest.ValueType.Kind == ir.TypeKindVoid {
			return "; unresolved dynamic/default call " + sym
		}
		destT := MapLLVMType(dest.ValueType)
		if destT == "i8*" {
			if strings.HasSuffix(sym, "_Init") && len(args) > 0 {
				p, v := e.coerceLLVM(args[0], "i8*", tag+"_init")
				return fmt.Sprintf("%s%s = bitcast i8* %s to i8*", p, e.formatValue(dest), v)
			}
			return fmt.Sprintf("%s = bitcast i8* null to i8*", e.formatValue(dest))
		}
		if destT == "double" {
			return fmt.Sprintf("%s = fadd double 0.0, 0.0", e.formatValue(dest))
		}
		if destT == "i1" {
			return fmt.Sprintf("%s = icmp ne i64 0, 0", e.formatValue(dest))
		}
		return fmt.Sprintf("%s = add %s 0, 0", e.formatValue(dest), destT)
	}
	if userFn != nil && len(args) < len(userFn.Params) {
		if len(args)+1 == len(userFn.Params) && (userFn.Params[0].Name == "this" || userFn.Params[0].Name == "$this") {
			args = append([]ir.Value{ir.NewConstNull(userFn.Params[0].Type())}, args...)
		} else {
			for len(args) < len(userFn.Params) {
				pT := userFn.Params[len(args)].Type()
				if pT.Kind == ir.TypeKindI64 || pT.Kind == ir.TypeKindBool {
					args = append(args, ir.NewConstInt(0, pT))
				} else {
					args = append(args, ir.NewConstNull(pT))
				}
			}
		}
	} else if userFn != nil && len(args) > len(userFn.Params) {
		if len(args) == len(userFn.Params)+1 && (len(userFn.Params) == 0 || (userFn.Params[0].Name != "this" && userFn.Params[0].Name != "$this")) {
			args = args[1:]
		} else {
			args = args[:len(userFn.Params)]
		}
	}
	extArgTs := e.symArgTypes[sym]
	isVararg := e.varargSyms != nil && e.varargSyms[sym]
	argCount := len(args)
	if userFn == nil && !isVararg && len(extArgTs) > argCount {
		argCount = len(extArgTs)
	}
	argStrs := make([]string, argCount)
	for idx := 0; idx < argCount; idx++ {
		if idx >= len(args) {
			switch extArgTs[idx] {
			case "i8*":
				argStrs[idx] = "i8* null"
			case "double":
				argStrs[idx] = "double 0.0"
			case "i1":
				argStrs[idx] = "i1 false"
			default:
				argStrs[idx] = extArgTs[idx] + " 0"
			}
			continue
		}
		a := args[idx]
		if userFn != nil && idx < len(userFn.Params) {
			paramLLVM := MapLLVMType(userFn.Params[idx].Type())
			p, expr := e.coerceLLVM(a, paramLLVM, fmt.Sprintf("%s_a%d", tag, idx))
			pre.WriteString(p)
			argStrs[idx] = fmt.Sprintf("%s %s", paramLLVM, expr)
		} else if !isVararg && idx < len(extArgTs) {
			p, expr := e.coerceLLVM(a, extArgTs[idx], fmt.Sprintf("%s_a%d", tag, idx))
			pre.WriteString(p)
			argStrs[idx] = fmt.Sprintf("%s %s", extArgTs[idx], expr)
		} else {
			argStrs[idx] = e.formatTypedValue(a)
		}
	}
	argsJoined := strings.Join(argStrs, ", ")
	sigPrefix := ""
	if e.varargSyms != nil && e.varargSyms[sym] {
		sigPrefix = "(...) "
	}
	if dest != nil && dest.ValueType.Kind != ir.TypeKindVoid {
		destLLVM := MapLLVMType(dest.ValueType)
		callRetLLVM := destLLVM
		if userFn != nil {
			callRetLLVM = MapLLVMType(userFn.ReturnType)
		} else if e.symRetTypes != nil && e.symRetTypes[sym] != "" {
			callRetLLVM = e.symRetTypes[sym]
		}
		if callRetLLVM == "void" {
			destName := e.formatValue(dest)
			zeroAssign := ""
			switch destLLVM {
			case "i8*":
				zeroAssign = fmt.Sprintf("\n\t%s = bitcast i8* null to i8*", destName)
			case "double":
				zeroAssign = fmt.Sprintf("\n\t%s = fadd double 0.0, 0.0", destName)
			case "i1":
				zeroAssign = fmt.Sprintf("\n\t%s = icmp ne i64 0, 0", destName)
			default:
				zeroAssign = fmt.Sprintf("\n\t%s = add %s 0, 0", destName, destLLVM)
			}
			return fmt.Sprintf("%scall void %s@%s(%s)%s", pre.String(), sigPrefix, sym, argsJoined, zeroAssign)
		}
		if callRetLLVM != destLLVM {
			rawTmp := fmt.Sprintf("%%raw_%s", tag)
			destName := e.formatValue(dest)
			conv := ""
			switch {
			case callRetLLVM == "i64" && destLLVM == "i8*":
				conv = fmt.Sprintf("\n\t%s = inttoptr i64 %s to i8*", destName, rawTmp)
			case callRetLLVM == "i8*" && destLLVM == "i64":
				conv = fmt.Sprintf("\n\t%s = ptrtoint i8* %s to i64", destName, rawTmp)
			case callRetLLVM == "i1" && destLLVM == "i64":
				conv = fmt.Sprintf("\n\t%s = zext i1 %s to i64", destName, rawTmp)
			case callRetLLVM == "i64" && destLLVM == "i1":
				conv = fmt.Sprintf("\n\t%s = icmp ne i64 %s, 0", destName, rawTmp)
			default:
				conv = fmt.Sprintf("\n\t%s = bitcast %s %s to %s", destName, callRetLLVM, rawTmp, destLLVM)
			}
			return fmt.Sprintf("%s%s = call %s %s@%s(%s)%s", pre.String(), rawTmp, callRetLLVM, sigPrefix, sym, argsJoined, conv)
		}
		return fmt.Sprintf("%s%s = call %s %s@%s(%s)", pre.String(), e.formatValue(dest), destLLVM, sigPrefix, sym, argsJoined)
	}
	retT := "void"
	if userFn != nil && userFn.ReturnType.Kind != ir.TypeKindVoid {
		retT = MapLLVMType(userFn.ReturnType)
	} else if e.symRetTypes != nil && e.symRetTypes[sym] != "" {
		retT = e.symRetTypes[sym]
	}
	return fmt.Sprintf("%scall %s %s@%s(%s)", pre.String(), retT, sigPrefix, sym, argsJoined)
}

func (e *LLVMEmitter) emitDirectCast(srcLLVM, srcVal, dstLLVM, dstVal string) string {
	switch {
	case srcLLVM == "i64" && dstLLVM == "i8*":
		return fmt.Sprintf("%s = inttoptr i64 %s to i8*", dstVal, srcVal)
	case srcLLVM == "i8*" && dstLLVM == "i64":
		return fmt.Sprintf("%s = ptrtoint i8* %s to i64", dstVal, srcVal)
	case srcLLVM == "i1" && dstLLVM == "i64":
		return fmt.Sprintf("%s = zext i1 %s to i64", dstVal, srcVal)
	case srcLLVM == "i64" && dstLLVM == "i1":
		return fmt.Sprintf("%s = icmp ne i64 %s, 0", dstVal, srcVal)
	case srcLLVM == "double" && dstLLVM == "i64":
		return fmt.Sprintf("%s = bitcast double %s to i64", dstVal, srcVal)
	case srcLLVM == "i64" && dstLLVM == "double":
		return fmt.Sprintf("%s = bitcast i64 %s to double", dstVal, srcVal)
	case srcLLVM == "i8*" && dstLLVM == "i1":
		return fmt.Sprintf("%s = icmp ne i8* %s, null", dstVal, srcVal)
	default:
		return fmt.Sprintf("%s = bitcast %s %s to %s", dstVal, srcLLVM, srcVal, dstLLVM)
	}
}

func formatLLVMStringLiteral(s string) (string, int) {
	var buf bytes.Buffer
	length := len(s) + 1 // + null terminator
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 32 && b <= 126 && b != '"' && b != '\\' {
			buf.WriteByte(b)
		} else {
			buf.WriteString(fmt.Sprintf("\\%02X", b))
		}
	}
	buf.WriteString("\\00")
	return buf.String(), length
}
