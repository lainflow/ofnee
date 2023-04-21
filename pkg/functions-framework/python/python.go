package python

import (
	"fmt"
	"os"
	"path/filepath"

	functions_framework "github.com/openfunction/pkg/functions-framework"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionFramework struct {
	RenderData    interface{}
	templateFiles []*functions_framework.FunctionFile
	staticFiles   map[string]string
}

const (
	templateMainPYPath    = "pkg/functions-framework/python/main.py.txt"
	functionContextPYPath = "pkg/functions-framework/python/function_context.py"
)

type renderData struct {
	UserCode     string
	FunctionName string
	Port         string
}

func CreatePythonFunctionFramework(name string, port int, userCode string) *FunctionFramework {
	dir, _ := os.Getwd()
	templateFile := filepath.Join(dir, templateMainPYPath)
	functionContextContext, _ := os.ReadFile(filepath.Join(dir, functionContextPYPath))
	data := renderData{
		UserCode:     userCode,
		FunctionName: name,
		Port:         fmt.Sprint(port),
	}
	mainPYFile := functions_framework.RegisterFile("main.py", data, templateFile)
	return &FunctionFramework{
		RenderData:    data,
		templateFiles: []*functions_framework.FunctionFile{mainPYFile},
		staticFiles: map[string]string{
			"function_context.py": string(functionContextContext),
		},
	}
}

func (ff *FunctionFramework) GenerateConfigMap(name string, namespace string) *v1.ConfigMap {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{},
	}
	for _, file := range ff.templateFiles {
		cmData := file.Render()
		cm.Data[file.Name] = string(cmData)
	}
	for fileName, content := range ff.staticFiles {
		cm.Data[fileName] = content
	}
	return cm
}
