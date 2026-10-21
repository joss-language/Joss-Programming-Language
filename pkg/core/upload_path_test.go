package core

import (
	"testing"
)

func TestUploadPathEvaluation(t *testing.T) {
	runtime := executeCode(t, `
var $file = {"name": "test.png", "type": "image/png"}
var $rawName = (!empty($file["name"])) ? $file["name"] : "image.png"
var $originalName = $rawName
(!Str::contains($originalName, ".")) ? {
    var $ext = ".png"
    var $mime = $file["type"] ?? ""
    (Str::contains($mime, "jpeg") || Str::contains($mime, "jpg")) ? { $ext = ".jpg" }
    (Str::contains($mime, "webp")) ? { $ext = ".webp" }
    (Str::contains($mime, "gif")) ? { $ext = ".gif" }
    (Str::contains($mime, "svg")) ? { $ext = ".svg" }
    $originalName = $originalName . $ext
}
var $ts = 1792620142
var $safeName = Str::replace(" ", "_", $originalName)
var $objectPath = "cms-media/" . $ts . "_" . $safeName
`)

	assertRuntimeValue(t, runtime, "rawName", "test.png")
	assertRuntimeValue(t, runtime, "originalName", "test.png")
	assertRuntimeValue(t, runtime, "safeName", "test.png")
	assertRuntimeValue(t, runtime, "objectPath", "cms-media/1792620142_test.png")
}
