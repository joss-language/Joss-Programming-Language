package core

import (
	"testing"
)

func TestUploadPathEvaluation(t *testing.T) {
	runtime := executeCode(t, `
var $file = {"name": "test.png", "type": "image/png"}
var $rawName = (!empty($file["name"])) ? $file["name"] : "image.png"
var $originalName = $rawName
(!str_contains($originalName, ".")) ? {
    var $ext = ".png"
    var $mime = $file["type"] ?? ""
    (str_contains($mime, "jpeg") || str_contains($mime, "jpg")) ? { $ext = ".jpg" }
    (str_contains($mime, "webp")) ? { $ext = ".webp" }
    (str_contains($mime, "gif")) ? { $ext = ".gif" }
    (str_contains($mime, "svg")) ? { $ext = ".svg" }
    $originalName = $originalName . $ext
}
var $ts = 1792620142
var $safeName = str_replace(" ", "_", $originalName)
var $objectPath = "cms-media/" . $ts . "_" . $safeName
`)

	assertRuntimeValue(t, runtime, "rawName", "test.png")
	assertRuntimeValue(t, runtime, "originalName", "test.png")
	assertRuntimeValue(t, runtime, "safeName", "test.png")
	assertRuntimeValue(t, runtime, "objectPath", "cms-media/1792620142_test.png")
}
