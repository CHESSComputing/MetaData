package main

import (
	"embed"
	"log"
	"time"

	srvConfig "github.com/CHESSComputing/golib/config"
	docdb "github.com/CHESSComputing/golib/docdb"
	lexicon "github.com/CHESSComputing/golib/lexicon"
	server "github.com/CHESSComputing/golib/server"
	utils "github.com/CHESSComputing/golib/utils"
	"github.com/gin-gonic/gin"
)

// content is our static web server content.
//
//go:embed static
var StaticFs embed.FS

// Verbose defines verbosity level
var Verbose int

// global variables
var _beamlines []string
var _smgr SchemaManager

// helper function to setup our router
func setupRouter() *gin.Engine {
	routes := []server.Route{
		server.Route{Method: "GET", Path: "/meta", Handler: MetaDetailsHandler, Authorized: false},
		server.Route{Method: "GET", Path: "/record", Handler: RecordHandler, Authorized: false},
		server.Route{Method: "GET", Path: "/records", Handler: RecordsHandler, Authorized: true},
		server.Route{Method: "PUT", Path: "/", Handler: DataHandler, Authorized: true, Scope: "write"},
		server.Route{Method: "POST", Path: "/", Handler: DataHandler, Authorized: true, Scope: "write"},
		server.Route{Method: "POST", Path: "/search", Handler: QueryHandler, Authorized: true},
		server.Route{Method: "POST", Path: "/summary", Handler: SummaryHandler, Authorized: false},
		server.Route{Method: "POST", Path: "/count", Handler: QueryCountHandler, Authorized: true},
		server.Route{Method: "DELETE", Path: "/record", Handler: DeleteHandler, Authorized: true, Scope: "delete"},
	}
	r := server.Router(routes, nil, "static", srvConfig.Config.CHESSMetaData.WebServer)
	return r
}

// Server defines our HTTP server
func Server() {
	// init docdb
	log.Println("init docdb", srvConfig.Config.CHESSMetaData.MongoDB.DBUri)
	docdb.InitDocDB(srvConfig.Config.CHESSMetaData.MongoDB.DBUri)

	// init Verbose
	Verbose = srvConfig.Config.CHESSMetaData.WebServer.Verbose
	if srvConfig.Config.CHESSMetaData.SchemaRenewInterval == 0 {
		SchemaRenewInterval = time.Duration(1 * 60 * 60 * time.Second) // by default renew every 1 hour
	} else {
		SchemaRenewInterval = time.Duration(srvConfig.Config.CHESSMetaData.SchemaRenewInterval) * time.Second
	}

	// initialize schema manager
	_smgr = SchemaManager{}
	for _, fname := range srvConfig.Config.CHESSMetaData.SchemaFiles {
		_, err := _smgr.Load(fname)
		if err != nil {
			log.Fatalf("unable to load %s error %v", fname, err)
		}
		_beamlines = append(_beamlines, utils.FileName(fname))
	}

	log.Println("Schema", _smgr.String())

	// load Lexicon patterns
	lexPatterns, err := lexicon.LoadPatterns(srvConfig.Config.CHESSMetaData.LexiconFile)
	if err != nil {
		log.Fatal(err)
	}
	lexicon.LexiconPatterns = lexPatterns

	_skipKeys = srvConfig.Config.CHESSMetaData.SkipKeys
	if len(_skipKeys) == 0 {
		// default list
		_skipKeys = []string{"user", "date", "description", "schema_name", "schema_file", "schema", "did", "doi", "doi_url"}
	}

	// setup web router and start the service
	r := setupRouter()
	webServer := srvConfig.Config.CHESSMetaData.WebServer
	server.StartServer(r, webServer)
}
