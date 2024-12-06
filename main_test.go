package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	srvConfig "github.com/CHESSComputing/golib/config"
	docdb "github.com/CHESSComputing/golib/docdb"
	"github.com/CHESSComputing/golib/lexicon"
	server "github.com/CHESSComputing/golib/server"
	utils "github.com/CHESSComputing/golib/utils"
	"github.com/gin-gonic/gin"
)

// helper function to initialize MetaData for tests
func initMetaData() {
	srvConfig.Init()
	log.SetFlags(0)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// current directory is a <pwd>/test
	_, err := os.Getwd()
	if err != nil {
		log.Fatal("unable to get current working dir")
	}
	// load Lexicon patterns
	lexPatterns, err := lexicon.LoadPatterns(srvConfig.Config.CHESSMetaData.LexiconFile)
	if err != nil {
		log.Fatal(err)
	}
	lexicon.LexiconPatterns = lexPatterns

	// init docdb
	log.Println("init docdb", srvConfig.Config.CHESSMetaData.MongoDB.DBUri)
	docdb.InitDocDB(srvConfig.Config.CHESSMetaData.MongoDB.DBUri)

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
}

var router *gin.Engine

func initServer() {
	if router == nil {
		// we need to initialize meta data only once since it calls srvConfig.Init()
		initMetaData()
	}
	if router == nil {
		routes := []server.Route{
			server.Route{Method: "GET", Path: "/:did", Handler: RecordHandler, Authorized: false},
			server.Route{Method: "PUT", Path: "/", Handler: DataHandler, Authorized: false},
			server.Route{Method: "POST", Path: "/", Handler: DataHandler, Authorized: false},
			server.Route{Method: "DELETE", Path: "/:did", Handler: DeleteHandler, Authorized: false},
		}
		router = server.Router(routes, nil, "static", srvConfig.Config.CHESSMetaData.WebServer)
	}
}

// helper function to print any struct in formatted way
func logStruct(t *testing.T, msg string, data any) {
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Logf("%s\n%+v\n", msg, data)
		return
	}
	t.Logf("%s\n%s\n", msg, string(body))
}

// helper function to create http test response recorder
// for given HTTP Method, endPoint, reader and web handler
func responseRecorder(t *testing.T, v TestCase) *httptest.ResponseRecorder {
	// read data from the inpit
	data, err := json.Marshal(v.Input)
	if err != nil {
		t.Fatal(err.Error())
	}
	reader := bytes.NewReader(data)

	if v.Verbose > 0 {
		t.Logf("submit method=%s endpoint=%s url=%s input=%v output=%v code=%v fail=%v data=%s", v.Method, v.Endpoint, v.Url, v.Input, v.Output, v.Code, v.Fail, string(data))
	}
	// setup HTTP request
	req, err := http.NewRequest(v.Method, v.Url, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Accept", "application/json")
	if v.Method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}

	// create response recorder
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if v.Verbose > 1 {
		logStruct(t, "HTTP request", req)
		logStruct(t, "HTTP response", rr)
	}
	return rr
}
