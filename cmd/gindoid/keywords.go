package main

import (
	"encoding/xml"
	"fmt"
	gdtmpl "github.com/G-Node/gin-doi/templates"
	"github.com/G-Node/libgin/libgin"
	"github.com/spf13/cobra"
	"html/template"
	"log"
	"os"
	"sort"
)

func mkkeywords(cmd *cobra.Command, args []string) {
	keywordMap := make(map[string][]*libgin.RepositoryMetadata) // map keywords to DOIs
	fmt.Println("Reading files")
	for idx, filearg := range args {
		var contents []byte
		var err error
		if isURL(filearg) {
			contents, err = readFileAtURL(filearg)
		} else {
			contents, err = readFileAtPath(filearg)
		}
		if err != nil {
			fmt.Printf("Failed to read file at %q: %s\n", filearg, err.Error())
			continue
		}

		datacite := new(libgin.DataCite)
		err = xml.Unmarshal(contents, datacite)
		if err != nil {
			fmt.Printf("Failed to unmarshal contents of %q: %s\n", filearg, err.Error())
			continue
		}
		metadata := &libgin.RepositoryMetadata{
			DataCite: datacite,
		}

		for _, kw := range metadata.Subjects {
			kw = KeywordURL(kw)
			datasets := keywordMap[kw]
			datasets = append(datasets, metadata)
			keywordMap[kw] = datasets
		}
		fmt.Printf(" %d/%d\r", idx+1, len(args))
	}

	fmt.Printf("\nFound %d keywords\n", len(keywordMap))
	fmt.Println("Creating pages")

	for kw, datasets := range keywordMap {
		tmpl, err := template.New(kw).Funcs(tmplfuncs).Parse(gdtmpl.Keyword)
		if err != nil {
			log.Printf("Could not parse the keyword page template: %s", err.Error())
			continue
		}

		os.MkdirAll(kw, 0777)
		fp, err := os.Create(fmt.Sprintf("%s/index.html", kw))
		if err != nil {
			log.Printf("Could not create the keyword page file: %s", err.Error())
			continue
		}
		defer fp.Close()
		data := make(map[string]interface{})
		data["Keyword"] = kw
		// Sort by date, lex order, which for ISO date strings should work fine
		sort.Slice(datasets, func(i, j int) bool {
			return datasets[i].Dates[0].Value > datasets[j].Dates[0].Value
		})
		data["Datasets"] = datasets
		if err := tmpl.Execute(fp, data); err != nil {
			log.Printf("Error rendering the keyword: %s", err.Error())
			continue
		}
		continue
	}
}
