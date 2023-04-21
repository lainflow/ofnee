package functions_framework

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"text/template"
)

type FunctionFile struct {
	Name     string
	Source   string
	Values   interface{}
	template string
}

func RegisterFile(name string, renderData interface{}, source string) *FunctionFile {
	return &FunctionFile{
		Name:   name,
		Source: source,
		Values: renderData,
	}
}

func (f *FunctionFile) generateTemplateFromSource() error {
	templateBytes, err := os.ReadFile(f.Source)
	if err != nil {
		log.Fatalf("failed to read template file %s: %v", f.Source, err)
		return err
	}
	f.template = string(templateBytes)
	return nil
}

func (f *FunctionFile) Render() []byte {
	if err := f.generateTemplateFromSource(); err != nil {
		return nil
	}

	tmpl, err := template.New(f.Name).Parse(f.template)
	if err != nil {
		fmt.Println("Failed to parse template:", err)
		return nil
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, f.Values)
	if err != nil {
		fmt.Println("Failed to execute template:", err)
		return nil
	}
	return buf.Bytes()
}
