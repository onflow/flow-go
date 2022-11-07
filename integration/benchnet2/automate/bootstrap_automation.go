package automate

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
)

type Template struct {
	template string
	jsonData string
}

func NewTemplate(jsonData string, templatePath string) Template {
	return Template{
		jsonData: jsonData,
		template: templatePath,
	}
}

func (t *Template) Apply(outputToFile bool) string {

	//load data values
	dataBytes, err := os.ReadFile(t.jsonData)
	if err != nil {
		log.Fatal(err)
	}

	// map json data to a map, so it can be decoded by template engine
	// https://stackoverflow.com/a/38437140/5719544
	var dataMap []map[string]interface{}
	if err := json.Unmarshal([]byte(dataBytes), &dataMap); err != nil {
		panic(err)
	}

	// load template
	templateBytes, err := os.ReadFile(t.template)
	if err != nil {
		log.Fatal(err)
	}
	templateStr := string(templateBytes)
	templateStr = strings.Trim(templateStr, "\t \n")

	tmpl, err := template.New("test").Parse(templateStr)
	tmpl = template.Must(tmpl, err)

	buf := new(strings.Builder)

	err = tmpl.Execute(buf, dataMap)
	if err != nil {
		log.Fatal(err)
	}

	//output := buf.String()

	if outputToFile {
		// create the file
		f, err := os.Create("values.yml")
		if err != nil {
			fmt.Println(err)
		}
		defer f.Close()

		_, e := f.WriteString(buf.String())
		if e != nil {
			log.Fatal(e)
		}
	}

	return buf.String()
}
