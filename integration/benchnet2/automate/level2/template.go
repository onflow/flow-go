package level2

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
	//load data values that will be applied against the template
	dataBytes, err := os.ReadFile(t.jsonData)
	if err != nil {
		log.Fatal(err)
	}

	// map any json data to array of maps, so it can be decoded by template engine -
	// this avoids the use of structs, so we can represent any arbitrary data
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

	helmTemplate, err := template.New("helm").Parse(templateStr)
	helmTemplate = template.Must(helmTemplate, err)

	buf := new(strings.Builder)

	err = helmTemplate.Execute(buf, dataMap)
	if err != nil {
		log.Fatal(err)
	}

	// remove any extra trailing white space
	trimmed := strings.TrimSpace(buf.String())

	if outputToFile {
		// create the file
		f, err := os.Create("values.yml")
		if err != nil {
			fmt.Println(err)
		}
		defer f.Close()

		_, e := f.WriteString(trimmed)
		if e != nil {
			log.Fatal(e)
		}
	}
	return trimmed
}
