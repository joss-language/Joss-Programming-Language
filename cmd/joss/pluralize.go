package main

import (
	"github.com/jossecurity/joss/pkg/core"
)

func pluralize(s string) string {
	return core.Pluralize(s)
}

func singularize(s string) string {
	return core.Singularize(s)
}
