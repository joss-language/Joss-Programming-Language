; ModuleID = 'test_grandb'
source_filename = "test_grandb.joss"

; String Constants
@.str.2 = private unnamed_addr constant [5 x i8] c"Joss\00", align 1
@.str.3 = private unnamed_addr constant [6 x i8] c"email\00", align 1
@.str.4 = private unnamed_addr constant [17 x i8] c"joss@example.com\00", align 1
@.str.1 = private unnamed_addr constant [5 x i8] c"name\00", align 1
@.str.0 = private unnamed_addr constant [6 x i8] c"users\00", align 1

; Declaraciones de Runtime Nativo Joss
declare void @joss_print_i64(i64)
declare void @joss_print_f64(double)
declare void @joss_print_bool(i1)
declare void @joss_print_string(i8*)
declare void @joss_panic(i8*)
declare i8* @joss_str_concat(i8*, i8*)
declare i8* @joss_obj_new(i8*)
declare void @joss_obj_set_field_i64(i8*, i8*, i64)
declare i64 @joss_obj_get_field_i64(i8*, i8*)
declare void @joss_obj_set_field_str(i8*, i8*, i8*)
declare i8* @joss_obj_get_field_str(i8*, i8*)
declare i8* @joss_arr_new(i64)
declare void @joss_arr_push_i64(i8*, i64)
declare void @joss_arr_push_str(i8*, i8*)
declare i64 @joss_arr_get_i64(i8*, i64)
declare i8* @joss_arr_get_str(i8*, i64)
declare void @joss_arr_set_i64(i8*, i64, i64)
declare void @joss_arr_set_str(i8*, i64, i8*)
declare i64 @joss_arr_len(i8*)
declare i8* @joss_map_new()
declare void @joss_map_set_i64(i8*, i8*, i64)
declare i64 @joss_map_get_i64(i8*, i8*)
declare void @joss_map_set_str(i8*, i8*, i8*)
declare i8* @joss_map_get_str(i8*, i8*)

define i64 @main() {
entry:
    %t0 = call i8* @GranDB_table(i8* getelementptr inbounds ([6 x i8], [6 x i8]* @.str.0, i64 0, i64 0))
    %t1 = alloca i8*, align 8
    store i8* %t0, i8** %t1, align 8
    %t2 = load i8*, i8** %t1, align 8
    %t3 = call i8* @joss_map_new()
	call void @joss_map_set_str(i8* %t3, i8* getelementptr inbounds ([5 x i8], [5 x i8]* @.str.1, i64 0, i64 0), i8* getelementptr inbounds ([5 x i8], [5 x i8]* @.str.2, i64 0, i64 0))
	call void @joss_map_set_str(i8* %t3, i8* getelementptr inbounds ([6 x i8], [6 x i8]* @.str.3, i64 0, i64 0), i8* getelementptr inbounds ([17 x i8], [17 x i8]* @.str.4, i64 0, i64 0))
    %t4 = call i64 @GranDB_insert(i8* %t2, i8* %t3)
    %t5 = load i8*, i8** %t1, align 8
    %t6 = call i8* @GranDB_where(i8* %t5, i8* getelementptr inbounds ([5 x i8], [5 x i8]* @.str.1, i64 0, i64 0), i8* getelementptr inbounds ([5 x i8], [5 x i8]* @.str.2, i64 0, i64 0))
    %t7 = call i8* @GranDB_first(i8* %t6)
    %t8 = alloca i8*, align 8
    store i8* %t7, i8** %t8, align 8
    %t9 = load i8*, i8** %t8, align 8
    %t10 = call i64 @joss_map_get_i64(i8* %t9, i8* getelementptr inbounds ([6 x i8], [6 x i8]* @.str.3, i64 0, i64 0))
    call void @joss_print_i64(i64 %t10)
    ret i64 0
}

