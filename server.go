package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	authz "github.com/CHESSComputing/golib/authz"
	srvConfig "github.com/CHESSComputing/golib/config"
	mongo "github.com/CHESSComputing/golib/mongo"
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
var _header, _footer string

// init function
// func init() {
// }

func header() string {
	if _header == "" {
		tmpl := server.MakeTmpl(StaticFs, "Header")
		tmpl["Base"] = srvConfig.Config.CHESSMetaData.WebServer.Base
		_header = server.TmplPage(StaticFs, "header.tmpl", tmpl)
	}
	return _header
}
func footer() string {
	if _footer == "" {
		tmpl := server.MakeTmpl(StaticFs, "Footer")
		tmpl["Base"] = srvConfig.Config.CHESSMetaData.WebServer.Base
		_footer = server.TmplPage(StaticFs, "footer.tmpl", tmpl)
	}
	return _footer
}

// helper function to handle base path of URL requests
func base(api string) string {
	b := srvConfig.Config.CHESSMetaData.WebServer.Base
	return utils.BasePath(b, api)
}

func setupRouter() *gin.Engine {
	// Disable Console Color
	// gin.DisableConsoleColor()
	r := gin.Default()

	// CHESSDataManagement APIs
	r.GET(base("/schemas"), SchemasHandler)
	r.GET(base("/process"), ProcessHandler)

	// TMP: until I implement tokens in client
	r.POST(base("/updateRecord"), UpdateRecordHandler)
	r.POST(base("/json"), UploadJsonHandler)
	// end of TMP

	// all POST methods ahould be authorized
	authorized := r.Group("/")
	authorized.Use(authz.TokenMiddleware(srvConfig.Config.Authz.ClientID, Verbose))
	{
		// data-service APIs
		authorized.GET("/:did", RecordHandler)
		authorized.PUT("/", UpdateHandler)
		authorized.POST("/", DataHandler)
		authorized.POST("/search", QueryHandler)
		authorized.DELETE("/:did", DeleteHandler)
	}

	// static files
	for _, dir := range []string{"js", "css", "images", "templates"} {
		filesFS, err := fs.Sub(StaticFs, "static/"+dir)
		if err != nil {
			log.Fatal(err)
		}
		m := fmt.Sprintf("%s/%s", srvConfig.Config.CHESSMetaData.WebServer.Base, dir)
		r.StaticFS(m, http.FS(filesFS))
	}

	// assign middleware
	r.Use(server.CounterMiddleware())

	return r
}

func Server() {
	// init MongoDB
	log.Println("init mongo", srvConfig.Config.CHESSMetaData.MongoDB.DBUri)
	mongo.InitMongoDB(srvConfig.Config.CHESSMetaData.MongoDB.DBUri)

	// init Verbose
	Verbose = srvConfig.Config.CHESSMetaData.WebServer.Verbose

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
	// setup web router and start the service
	r := setupRouter()
	sport := fmt.Sprintf(":%d", srvConfig.Config.CHESSMetaData.WebServer.Port)
	log.Printf("Start HTTP server %s", sport)
	r.Run(sport)
}
