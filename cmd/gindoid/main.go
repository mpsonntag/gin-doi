package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/docopt/docopt-go"
)

func main() {
	usage := `gindoid: DOI service for preparing GIN repositories for publication
Usage:
  gindoid [--max_workers=<max_workers> --max_queue_size=<max_queue_size> --port=<port> --source=<source>
           --gitsource=<gitdsourceurl>
           --oauthserver=<oserv> --target=<target> --storeURL=<url> --mServer=<server> --mFrom=<from>
           --doiMaster=<master> --doiBase=<base> --sendMail --debug --templates=<tmplpath> --scpURL=<scpURL>] --key=<key>

Options:
  --max_workers=<max_workers>     The number of workers to start [default: 3]
  --max_queue_size=<max_quesize>  The size of the job queue [default: 100]
  --port=<port>                   The server port [default: 8083]
  --source=<dsourceurl>           The server address from which data can be read [default: https://web.gin.g-node.org]
  --gitsource=<gitdsourceurl>     The git server address from which data can be cloned [default: ssh://git@gin.g-node.org]
  --oauthserver=<repo>            The server of the repo service [default: https://web.gin.g-node.org]
  --target=<target>               The location for long term storage [default: data]
  --storeURL=<url>                The base URL for storage [default: http://doid.gin.g-node.org/]
  --mServer=<server>              The mail server address (:and port) [default: localhost:25]
  --mFrom=<from>                  The mail from address [default: no-reply@g-node.org]
  --doiMaster=<master>            The mail address to send info to [default: dev@g-node.org]
  --doiBase=<base>                The DOI prefix [default: 10.12751/g-node.]
  --sendMail                      Whether mail notifications should really be sent (otherwise just print them)
  --debug                         Whether debug messages shall be printed
  --templates=<tmplpath>          Path to the templates [default: tmpl]
  --scpURL=<scpURL>               URI for SCP copying of the datacite XML [default: gin.g-node.org:/data/doid]
  --key=<key>                     Key used to decrypt token
 `

	args, err := docopt.Parse(usage, nil, true, "gin doi 0.1a", false)
	if err != nil {
		log.Printf("Error while parsing command line: %+v", err)
		os.Exit(-1)
	}
	// Setup data source
	ds := &GogsDataSource{GinURL: args["--source"].(string), GinGitURL: args["--gitsource"].(string)}

	// doi provider
	dp := GnodeDoiProvider{ApiURI: "", DOIBase: args["--doiBase"].(string)}

	//Setup storage
	mServer := MailServer{Adress: args["--mServer"].(string), From: args["--mFrom"].(string),
		DoSend: args["--sendMail"].(bool),
		Master: args["--doiMaster"].(string)}
	storage := LocalStorage{Path: args["--target"].(string), Source: ds, HttpBase: args["--storeURL"].(string),
		DProvider: dp, MServer: &mServer, TemplatePath: args["--templates"].(string),
		SCPURL: args["--scpURL"].(string)}

	// setup authentication
	oaAdress := args["--oauthserver"].(string)
	op := GogsOauthProvider{
		Uri:      fmt.Sprintf("%s/api/v1/user", oaAdress),
		TokenURL: "",
		KeyURL:   fmt.Sprintf("%s/api/v1/user/keys", oaAdress),
	}

	key := args["--key"].(string)

	// Create the job queue.
	maxQ, err := strconv.Atoi(args["--max_queue_size"].(string))
	if err != nil {
		log.Printf("Error while parsing command line: %+v", err)
		os.Exit(-1)
	}
	jobQueue := make(chan DoiJob, maxQ)
	// Start the dispatcher.
	maxW, err := strconv.Atoi(args["--max_workers"].(string))
	dispatcher := NewDispatcher(jobQueue, maxW)
	dispatcher.Run(NewWorker)

	// Start the HTTP handlers.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		InitDoiJob(w, r, ds, &op, storage.TemplatePath, &storage, key)
	})
	http.HandleFunc("/do/", func(w http.ResponseWriter, r *http.Request) {
		DoDoiJob(w, r, jobQueue, storage, &op)
	})
	http.Handle("/assets/",
		http.StripPrefix("/assets/", http.FileServer(http.Dir("/assets"))))

	//Debugging?
	if args["--debug"].(bool) {
		log.SetLevel(log.DebugLevel)
		log.SetFormatter(&log.TextFormatter{ForceColors: true})
	}

	log.Fatal(http.ListenAndServe(":"+args["--port"].(string), nil))
}
