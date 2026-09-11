package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

func createCRUD(tableName string) error {
	if isConsoleProject() {
		return fmt.Errorf("make:crud requiere un proyecto web con app/views")
	}
	if err := validateDatabaseIdentifier(tableName); err != nil {
		return err
	}
	fmt.Printf("Generating CRUD for table '%s'...\n", tableName)

	// 1. Connect to DB
	dbType, _, _, _, _, _, prefix := loadEnvConfig()
	db, err := connectToDB(dbType, readEnvFile(GetEnvFile()))

	if err != nil {
		return fmt.Errorf("conectando a la base de datos: %w", err)
	}
	defer db.Close()

	// 2. Inspect Schema
	cols, err := getColumns(db, dbType, tableName)
	if err != nil {
		return fmt.Errorf("inspeccionando tabla %s: %w", tableName, err)
	}

	// If table not found and doesn't start with prefix, try adding prefix
	if len(cols) == 0 && !strings.HasPrefix(tableName, prefix) {
		prefixedName := prefix + tableName
		fmt.Printf("Table '%s' not found. Trying '%s'...\n", tableName, prefixedName)
		cols, err = getColumns(db, dbType, prefixedName)
		if err != nil {
			return fmt.Errorf("inspeccionando tabla %s: %w", prefixedName, err)
		}
		if len(cols) > 0 {
			tableName = prefixedName
			fmt.Printf("Found table '%s'. Using it.\n", tableName)
		}
	}

	if len(cols) == 0 {
		return fmt.Errorf("la tabla %q no existe o no tiene columnas", tableName)
	}
	// 3. Analyze Relations
	var relations []Relation
	for _, c := range cols {
		fmt.Printf("Inspecting column: '%s'\n", c.Name)
		if strings.HasSuffix(c.Name, "_id") {
			fmt.Printf("  -> Found relation for %s\n", c.Name)
			// Infer relation
			baseName := strings.TrimSuffix(c.Name, "_id")
			_, _, _, _, _, _, prefix := loadEnvConfig()
			relatedTable := prefix + strings.ToLower(pluralize(baseName)) // Convention: js_users

			// Smartly detect display column
			displayCol, exists, displayErr := getDisplayColumn(db, dbType, relatedTable)
			if displayErr != nil {
				return fmt.Errorf("inspeccionando relacion %s: %w", relatedTable, displayErr)
			}
			if !exists {
				fmt.Printf("  -> Related table %s not found; treating %s as a regular field.\n", relatedTable, c.Name)
				continue
			}
			fmt.Printf("  -> Detected display column for %s: %s\n", relatedTable, displayCol)

			relations = append(relations, Relation{
				ForeignKey: c.Name,
				Table:      relatedTable,
				Alias:      baseName + "_" + displayCol, // e.g. user_username
				DisplayCol: displayCol,
			})
		}
	}
	fmt.Printf("Total relations found: %d\n", len(relations))

	// 4. Generate Artifacts
	// Model
	modelName := snakeToCamel(tableName)
	// Strip prefix
	camelPrefix := snakeToCamel(prefix)
	modelName = strings.TrimPrefix(modelName, camelPrefix)
	// Use singularize helper
	modelName = strings.Title(singularize(modelName))

	// Model
	createModel(modelName)

	// Auto-create related models
	for _, rel := range relations {
		relModelName := snakeToCamel(rel.Table)
		relModelName = strings.TrimPrefix(relModelName, camelPrefix)
		relModelName = strings.Title(singularize(relModelName))

		path := filepath.Join("app", "models", relModelName+".joss")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("Auto-creating missing related model: %s\n", relModelName)
			createModel(relModelName)
		}
	}

	// Controller
	createCRUDController(modelName, tableName, cols, relations)

	// Views
	createCRUDViews(modelName, cols, relations)
	updateNavbar(modelName)
	injectProtectedRoutes(modelName)
	return nil
}

func createCRUDController(modelName, tableName string, cols []ColumnSchema, relations []Relation) {
	path := filepath.Join("app", "controllers", modelName+"Controller.joss")
	os.MkdirAll(filepath.Dir(path), 0755)

	viewPrefix := strings.ToLower(modelName)

	// Build Index Query with Joins
	// Note: GranDB now handles prefixes automatically for select() and joins()
	indexLogic := fmt.Sprintf(`$%s = new %s()`, strings.ToLower(modelName), modelName)

	if len(relations) > 0 {
		indexLogic += fmt.Sprintf("\n        $data = $%s", strings.ToLower(modelName))

		// Selects
		// Use base table names, ORM will prefix them
		_, _, _, _, _, _, currentPrefix := loadEnvConfig()
		baseTableName := strings.TrimPrefix(tableName, currentPrefix)

		selects := []string{fmt.Sprintf("\"%s.*\"", baseTableName)}

		for _, rel := range relations {
			baseRelTable := strings.TrimPrefix(rel.Table, currentPrefix)
			selects = append(selects, fmt.Sprintf("\"%s.%s as %s\"", baseRelTable, rel.DisplayCol, rel.Alias))
		}
		indexLogic += fmt.Sprintf("->select([%s])", strings.Join(selects, ", "))

		// Joins
		for _, rel := range relations {
			baseRelTable := strings.TrimPrefix(rel.Table, currentPrefix)
			indexLogic += fmt.Sprintf("->leftJoin(\"%s\", \"%s.%s\", \"=\", \"%s.id\")", baseRelTable, baseTableName, rel.ForeignKey, baseRelTable)
		}
		indexLogic += "->get()"
	} else {
		indexLogic += fmt.Sprintf("\n        $data = $%s->get()", strings.ToLower(modelName))
	}

	// Build Create Logic (Fetch relations)
	createLogic := ""
	createVars := ""
	if len(relations) > 0 {
		for _, rel := range relations {
			// Derive model name from table: js_roles -> Role
			relModel := snakeToCamel(rel.Table)
			// Get current prefix to strip
			_, _, _, _, _, _, prefix := loadEnvConfig()
			camelPrefix := snakeToCamel(prefix)
			relModel = strings.TrimPrefix(relModel, camelPrefix)
			relModel = strings.Title(singularize(relModel))
			varName := strings.ToLower(pluralize(relModel)) // roles
			createLogic += fmt.Sprintf("\n        $%sModel = new %s()", strings.ToLower(relModel), relModel)
			createLogic += fmt.Sprintf("\n        $%s = $%sModel->get()", varName, strings.ToLower(relModel))
			createVars += fmt.Sprintf(", \"%s\": $%s", varName, varName)
		}
	}

	requestData := generateRequestData(cols)
	content := fmt.Sprintf(`public class %sController {
    
    public func index() {
        %s
        return view("%s.index", {"items": $data})
    }

    public func create() {
        %s
        return view("%s.create", {%s})
    }

    public func store() {
        $model = new %s()
        $data = %s
        $model->insert($data)
        return redirect("/%s")->with("success", "%s creado correctamente.")
    }

    public func edit(mixed $id) {
        $model = new %s()
        $item = $model->where("id", $id)->first()
        (!$item) ? {
            return redirect("/%s")->with("error", "Registro no encontrado.")
        }
        %s
        return view("%s.edit", {"item": $item%s})
    }

    public func update(mixed $id) {
        $model = new %s()
        $data = %s
        $model->where("id", $id)->update($data)
        return redirect("/%s")->with("success", "%s actualizado correctamente.")
    }

    public func delete(mixed $id) {
        $model = new %s()
        $model->where("id", $id)->delete()
        return redirect("/%s")->with("success", "%s eliminado correctamente.")
    }
}`, modelName, indexLogic, viewPrefix, createLogic, viewPrefix, strings.TrimPrefix(createVars, ", "),
		modelName, requestData, viewPrefix, modelName,
		modelName, viewPrefix, createLogic, viewPrefix, createVars,
		modelName, requestData, viewPrefix, modelName,
		modelName, viewPrefix, modelName)

	writeGenFile(path, content)
}

func generateRequestData(cols []ColumnSchema) string {
	fields := make([]string, 0, len(cols))
	for _, column := range cols {
		if column.Name == "id" || column.Name == "created_at" || column.Name == "updated_at" {
			continue
		}
		fields = append(fields, fmt.Sprintf(`"%s": Request::input("%s")`, column.Name, column.Name))
	}
	return "{" + strings.Join(fields, ", ") + "}"
}

func createCRUDViews(modelName string, cols []ColumnSchema, relations []Relation) {
	folder := filepath.Join("app", "views", strings.ToLower(modelName))
	os.MkdirAll(folder, 0755)

	// Index
	indexHtml := fmt.Sprintf(`@extends('layouts.master')

@section('content')
<div class="bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-sm overflow-hidden">
    <div class="flex justify-between items-center p-6 border-b border-gray-100 dark:border-gray-700">
        <h2 class="text-xl font-bold text-gray-950 dark:text-white">%s List</h2>
        <a href="/%s/create" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-xs font-bold text-white rounded-xl shadow-lg shadow-blue-500/20 transition"><i class="fas fa-plus mr-1"></i> Create New</a>
    </div>
    <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-100 dark:divide-gray-700">
            <thead class="bg-gray-50 dark:bg-gray-900/50">
                <tr>
                    %s
                    <th class="px-6 py-3 text-right text-xs font-bold text-gray-500 dark:text-gray-400 uppercase tracking-wider">Actions</th>
                </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-gray-750">
                @foreach($items as $item)
                <tr class="hover:bg-gray-50/50 dark:hover:bg-gray-750/30 transition">
                    %s
                    <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2">
                        <a href="/%s/edit/{{ $item.id }}" class="text-blue-500 hover:text-blue-600 transition"><i class="fas fa-edit"></i></a>
                        <form action="/%s/delete/{{ $item.id }}" method="POST" class="inline" onsubmit="return confirm('Are you sure?')">
                            {{ csrf_field() }}
                            <button type="submit" class="text-red-500 hover:text-red-650 transition"><i class="fas fa-trash"></i></button>
                        </form>
                    </td>
                </tr>
                @endforeach
            </tbody>
        </table>
    </div>
</div>
@endsection
`, modelName, strings.ToLower(modelName), generateIndexHeaders(cols, relations), generateIndexRows(cols, relations), strings.ToLower(modelName), strings.ToLower(modelName))

	writeGenFile(filepath.Join(folder, "index.joss.html"), indexHtml)

	// Create
	createHtml := fmt.Sprintf(`@extends('layouts.master')

@section('content')
<div class="max-w-2xl mx-auto bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-xl overflow-hidden p-8">
    <div class="text-center mb-8 border-b border-gray-100 dark:border-gray-700 pb-5">
        <h2 class="text-2xl font-bold text-gray-950 dark:text-white">Create %s</h2>
    </div>

    <form action="/%s/store" method="POST" class="space-y-5">
        {{ csrf_field() }}
%s
        <div class="flex justify-end gap-3 pt-4 border-t border-gray-100 dark:border-gray-700">
            <a href="/`+strings.ToLower(modelName)+`" class="px-5 py-2.5 text-sm font-bold text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-white transition">Cancel</a>
            <button type="submit" class="px-6 py-2.5 text-sm font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl shadow-lg shadow-blue-500/20 transition">Save Record</button>
        </div>
    </form>
</div>
@endsection
`, modelName, strings.ToLower(modelName), generateFormFields(cols, relations, false))
	writeGenFile(filepath.Join(folder, "create.joss.html"), createHtml)

	// Edit
	editHtml := fmt.Sprintf(`@extends('layouts.master')

@section('content')
<div class="max-w-2xl mx-auto bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-2xl shadow-xl overflow-hidden p-8">
    <div class="text-center mb-8 border-b border-gray-100 dark:border-gray-700 pb-5">
        <h2 class="text-2xl font-bold text-gray-950 dark:text-white">Edit %s</h2>
    </div>

    <form action="/%s/update/{{ $item.id }}" method="POST" class="space-y-5">
        {{ csrf_field() }}
%s
        <div class="flex justify-end gap-3 pt-4 border-t border-gray-100 dark:border-gray-700">
            <a href="/`+strings.ToLower(modelName)+`" class="px-5 py-2.5 text-sm font-bold text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-white transition">Cancel</a>
            <button type="submit" class="px-6 py-2.5 text-sm font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl shadow-lg shadow-blue-500/20 transition">Update Record</button>
        </div>
    </form>
</div>
@endsection
`, modelName, strings.ToLower(modelName), generateFormFields(cols, relations, true))
	writeGenFile(filepath.Join(folder, "edit.joss.html"), editHtml)
}

// Helpers for view generation to keep createCRUDViews somewhat clean
func generateIndexHeaders(cols []ColumnSchema, relations []Relation) string {
	var html string
	for _, c := range cols {
		headerName := c.Name
		for _, rel := range relations {
			if c.Name == rel.ForeignKey {
				headerName = strings.Title(strings.Replace(rel.Alias, "_", " ", -1))
				break
			}
		}
		html += fmt.Sprintf("                    <th class=\"px-6 py-3 text-left text-xs font-bold text-gray-500 dark:text-gray-400 uppercase tracking-wider\">%s</th>\n", headerName)
	}
	return html
}

func generateIndexRows(cols []ColumnSchema, relations []Relation) string {
	var html string
	for _, c := range cols {
		val := fmt.Sprintf("{{ $item.%s }}", c.Name)
		for _, rel := range relations {
			if c.Name == rel.ForeignKey {
				val = fmt.Sprintf("{{ $item.%s }}", rel.Alias)
				break
			}
		}
		html += fmt.Sprintf("                    <td class=\"px-6 py-4 whitespace-nowrap text-sm text-gray-700 dark:text-gray-300\">%s</td>\n", val)
	}
	return html
}

func generateFormFields(cols []ColumnSchema, relations []Relation, isEdit bool) string {
	var html string

	for _, c := range cols {
		if c.Name == "id" || c.Name == "created_at" || c.Name == "updated_at" {
			continue
		}

		// Check for relation
		isRelation := false
		var relData Relation
		for _, rel := range relations {
			if c.Name == rel.ForeignKey {
				isRelation = true
				relData = rel
				break
			}
		}

		if isRelation {
			// Derive variable name: js_roles -> roles
			relModel := snakeToCamel(relData.Table)
			_, _, _, _, _, _, prefix := loadEnvConfig()
			camelPrefix := snakeToCamel(prefix)
			relModel = strings.TrimPrefix(relModel, camelPrefix)
			relModel = strings.Title(singularize(relModel))
			varName := strings.ToLower(pluralize(relModel))

			selectedValue := ""
			if isEdit {
				selectedValue = fmt.Sprintf("{{ $item.%s == $opt.id ? 'selected' : '' }}", c.Name)
			}

			html += fmt.Sprintf(`        <div>
            <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">%s</label>
            <select name="%s" class="w-full p-2.5 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm">
                <option value="">Select %s</option>
                @foreach($%s as $opt)
                <option value="{{ $opt.id }}" %s>{{ $opt.%s }}</option>
                @endforeach
            </select>
        </div>
`, strings.Title(strings.Replace(c.Name, "_", " ", -1)), c.Name, relModel, varName, selectedValue, relData.DisplayCol)
		} else {
			valueAttr := ""
			if isEdit {
				valueAttr = fmt.Sprintf(" value=\"{{ $item.%s }}\"", c.Name)
			}
			html += fmt.Sprintf(`        <div>
            <label class="block mb-1.5 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">%s</label>
            <input type="text" name="%s" class="w-full p-2.5 bg-gray-50 border border-gray-200 dark:bg-gray-900/50 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 text-sm"%s>
        </div>
`, c.Name, c.Name, valueAttr)
		}
	}
	return html
}

func updateNavbar(modelName string) {
	// Try to find layouts/master.joss.html
	path := filepath.Join("app", "views", "layouts", "master.joss.html")
	content, err := os.ReadFile(path)
	if err == nil {
		html := string(content)

		// Try new Tailwind navbar format first
		newLink := fmt.Sprintf(`<li><a href="/%s" class="block py-2 px-3 text-gray-300 hover:text-white"><i class="fas fa-circle mr-1 text-xs"></i> %s</a></li>`, strings.ToLower(modelName), modelName)
		oldLink := fmt.Sprintf(`<li><a href="/%s"><i class="fas fa-circle"></i> %s</a></li>`, strings.ToLower(modelName), modelName)
		if strings.Contains(html, fmt.Sprintf(`href="/%s"`, strings.ToLower(modelName))) {
			fmt.Printf("Navbar already contains /%s.\n", strings.ToLower(modelName))
			return
		}

		injected := false
		// Insert before <!-- Injected Links Here --> (both formats)
		for _, marker := range []string{"<!-- Injected Links Here -->", "<!-- Injected Links Here-->"} {
			if strings.Contains(html, marker) {
				html = strings.Replace(html, marker, newLink+"\n                    "+marker, 1)
				injected = true
				break
			}
		}

		// Fallback: try old sidebar format
		if !injected {
			_ = oldLink
			fmt.Println("Warning: Could not find '<!-- Injected Links Here -->' marker in master.joss.html. Add it manually to your navbar.")
			return
		}

		os.WriteFile(path, []byte(html), 0644)
		fmt.Printf("Updated navbar in layouts/master.joss.html with link to /%s\n", strings.ToLower(modelName))
	}
}

func injectProtectedRoutes(modelName string) {
	path := "routes.joss"
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	routes := fmt.Sprintf(`
    // CRUD Routes for %s
    Router::get("/%s", "%sController@index")
    Router::get("/%s/create", "%sController@create")
    Router::post("/%s/store", "%sController@store")
    Router::get("/%s/edit/{id}", "%sController@edit")
    Router::post("/%s/update/{id}", "%sController@update")
    Router::post("/%s/delete/{id}", "%sController@delete")
`, modelName,
		strings.ToLower(modelName), modelName,
		strings.ToLower(modelName), modelName,
		strings.ToLower(modelName), modelName,
		strings.ToLower(modelName), modelName,
		strings.ToLower(modelName), modelName,
		strings.ToLower(modelName), modelName)

	strContent := string(content)
	if strings.Contains(strContent, fmt.Sprintf("// CRUD Routes for %s", modelName)) {
		fmt.Printf("Routes for %s already exist.\n", modelName)
		return
	}

	// Check if "auth" middleware group exists (Router::middleware("auth") ... Router::end())
	authMarker := `Router::middleware("auth")`
	endMarker := `Router::end()`
	if strings.Contains(strContent, authMarker) {
		// Inject inside the last auth middleware block, before its Router::end()
		lastEnd := strings.LastIndex(strContent, endMarker)
		if lastEnd != -1 {
			strContent = strContent[:lastEnd] + routes + strContent[lastEnd:]
			os.WriteFile(path, []byte(strContent), 0644)
			fmt.Println("Injected protected routes into 'auth' group.")
			return
		}
	}

	// If no auth group, append a new protected middleware block
	newGroup := fmt.Sprintf(`
Router::middleware("auth")
%sRouter::end()
`, routes)

	os.WriteFile(path, []byte(strContent+newGroup), 0644)
	fmt.Println("Created new 'auth' group with routes.")
}

func removeCRUD(tableName string) {
	fmt.Printf("Removing CRUD for table '%s'...\n", tableName)

	// 1. Infer Model Name
	modelName := snakeToCamel(tableName)
	// Strip prefix
	_, _, _, _, _, _, prefix := loadEnvConfig()
	camelPrefix := snakeToCamel(prefix)
	modelName = strings.TrimPrefix(modelName, camelPrefix)
	modelName = strings.Title(singularize(modelName))

	fmt.Printf("Inferred Model Name: %s\n", modelName)

	// 2. Delete Controller (recursively search app/controllers)
	controllerFileName := modelName + "Controller.joss"
	_ = filepath.Walk(filepath.Join("app", "controllers"), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.EqualFold(info.Name(), controllerFileName) {
			os.Remove(path)
			fmt.Printf("Deleted: %s\n", path)
		}
		return nil
	})

	// 3. Delete Model (recursively search app/models)
	modelFileName := modelName + ".joss"
	_ = filepath.Walk(filepath.Join("app", "models"), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.EqualFold(info.Name(), modelFileName) {
			os.Remove(path)
			fmt.Printf("Deleted: %s\n", path)
		}
		return nil
	})

	// 4. Delete Views
	viewsPath := filepath.Join("app", "views", strings.ToLower(modelName))
	if _, err := os.Stat(viewsPath); err == nil {
		os.RemoveAll(viewsPath)
		fmt.Printf("Deleted: %s\n", viewsPath)
	}

	// 5. Remove Routes
	routesPath := "routes.joss"
	content, err := os.ReadFile(routesPath)
	if err == nil {
		lines := strings.Split(string(content), "\n")
		var newLines []string
		for _, line := range lines {
			// Check for CRUD Routes comment
			if strings.Contains(line, fmt.Sprintf("// CRUD Routes for %s", modelName)) {
				continue
			}
			// Filter out lines that contain the controller name (case insensitive check)
			lowerLine := strings.ToLower(line)
			lowerController := strings.ToLower(modelName + "Controller")
			if strings.Contains(lowerLine, lowerController) {
				continue
			}
			newLines = append(newLines, line)
		}
		os.WriteFile(routesPath, []byte(strings.Join(newLines, "\n")), 0644)
		fmt.Println("Cleaned routes.")
	}

	// 6. Remove Navbar Link
	masterPath := filepath.Join("app", "views", "layouts", "master.joss.html")
	content, err = os.ReadFile(masterPath)
	if err == nil {
		lines := strings.Split(string(content), "\n")
		var newLines []string
		linkTarget := fmt.Sprintf(`href="/%s"`, strings.ToLower(modelName))
		for _, line := range lines {
			if strings.Contains(line, linkTarget) {
				continue
			}
			newLines = append(newLines, line)
		}
		os.WriteFile(masterPath, []byte(strings.Join(newLines, "\n")), 0644)
		fmt.Println("Cleaned navbar.")
	}

	fmt.Println("CRUD removal complete.")
}
