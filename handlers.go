package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	srvConfig "github.com/CHESSComputing/common/config"
	mongo "github.com/CHESSComputing/common/mongo"
	server "github.com/CHESSComputing/common/server"
	utils "github.com/CHESSComputing/common/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SiteParams represents URI storage params in /meta/:site end-point
type SiteParams struct {
	Site string `uri:"site" binding:"required"`
}

// MetaIdParams represents URI storage params in /meta/:mid end-point
type MetaIdParams struct {
	ID string `uri:"mid" binding:"required"`
}

// MetaHandler provives access to GET /meta end-point
func MetaHandler(c *gin.Context) {
	data := metadata("")
	c.JSON(200, gin.H{"status": "ok", "data": data})
}

// MetaSiteHandler provides access to GET /meta/:site end-point
func MetaSiteHandler(c *gin.Context) {
	var params SiteParams
	if err := c.ShouldBindUri(&params); err == nil {
		data := metadata(params.Site)
		c.JSON(200, gin.H{"status": "ok", "data": data})
	} else {
		c.JSON(400, gin.H{"status": "fail", "error": err.Error()})
	}
}

// MetaRecordHandler provides access to GET /meta/:site end-point
func MetaRecordHandler(c *gin.Context) {
	var params MetaIdParams
	if err := c.ShouldBindUri(&params); err == nil {
		data := getRecord(params.ID)
		c.JSON(200, gin.H{"status": "ok", "data": data})
	} else {
		c.JSON(400, gin.H{"status": "fail", "error": err.Error()})
	}
}

// MetaPostHandler provides access to POST /meta end-point
func MetaPostHandler(c *gin.Context) {
	var meta MetaData
	err := c.BindJSON(&meta)
	if err == nil {
		if meta.ID == "" {
			if uuid, err := uuid.NewRandom(); err == nil {
				meta.ID = hex.EncodeToString(uuid[:])
			}
		}
		_metaData = append(_metaData, meta)
		// upsert into MongoDB
		if srvConfig.Config.CHESSMetaData.MongoDB.DBUri != "" {
			//         meta.mongoUpsert("ID")
			meta.mongoInsert()
		}
		c.JSON(200, gin.H{"status": "ok"})
	} else {
		c.JSON(400, gin.H{"status": "fail", "error": err.Error()})
	}
}

// MetaDeleteHandler provides access to Delete /meta/:mid end-point
func MetaDeleteHandler(c *gin.Context) {
	var params MetaIdParams
	if err := c.ShouldBindUri(&params); err == nil {
		var metaData []MetaData
		for _, meta := range _metaData {
			if meta.ID != params.ID {
				metaData = append(metaData, meta)
				// remove record from MongoDB
				meta.mongoRemove()
			}
		}
		if len(_metaData) == len(metaData) {
			// record was not found
			msg := fmt.Sprintf("record %s was not found in MetaData service", params.ID)
			c.JSON(400, gin.H{"status": "fail", "error": msg})
			return
		}
		_metaData = metaData
		c.JSON(200, gin.H{"status": "ok"})
	} else {
		c.JSON(400, gin.H{"status": "fail", "error": err.Error()})
	}
}

// CHESSDataManagement APIs

// SchemasHandler handlers /schemas requests
func SchemasHandler(c *gin.Context) {
	var records []mongo.Record
	for _, sname := range srvConfig.Config.CHESSMetaData.SchemaFiles {
		file, err := os.Open(sname)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			log.Println("unable to open", sname, err)
		}
		var rec []mongo.Record
		err = json.Unmarshal(body, &rec)
		if err != nil {
			log.Println("unable to unmarshal body", err)
		}
		srec := make(mongo.Record)
		srec["schema"] = sname
		srec["records"] = rec
		records = append(records, srec)
	}
	c.JSON(200, records)
}

// SearchHandler handlers Search requests
func SearchHandler(c *gin.Context) {
	var err error
	var user string
	/*
		if Config.TestMode {
			user = "test"
			err = nil
		} else {
			user, err = username(r)
		}
		if err != nil {
			_, err := getUserCredentials(r)
			if err != nil {
				msg := "unable to get user credentials"
				handleError(w, r, msg, err)
				return
			}
		}
	*/

	w := c.Writer
	r := c.Request

	// create search template form
	tmpl := server.MakeTmpl(c, "Search")

	// if we got GET request it is /search web form
	if r.Method == "GET" {
		tmpl["Query"] = ""
		tmpl["User"] = user
		page := utils.TmplPage(StaticFs, "searchform.tmpl", tmpl)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(server.Top(c) + page + server.Bottom(c)))
		return
	}

	// if we get POST request we'll process user query
	query := r.FormValue("query")
	spec, err := ParseQuery(query)
	if Verbose > 0 {
		log.Printf("search query='%s' spec=%+v user=%v", query, spec, user)
	}
	if err != nil {
		msg := "unable to parse user query"
		server.HandleError(c, msg, err)
		return
	}

	// check if we use web or cli
	if client := r.FormValue("client"); client == "cli" {
		var records []mongo.Record
		if spec != nil {
			records = mongo.Get(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec, 0, -1)
		}
		c.JSON(200, records)
		return
	}
	// get form parameters
	limit, err := strconv.Atoi(r.FormValue("limit"))
	if err != nil {
		limit = 50
	}
	idx, err := strconv.Atoi(r.FormValue("idx"))
	if err != nil {
		idx = 0
	}

	tmpl["Query"] = query
	tmpl["User"] = user
	page := utils.TmplPage(StaticFs, "searchform.tmpl", tmpl)

	// process the query
	if spec != nil {
		nrec := mongo.Count(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec)
		records := mongo.Get(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec, 0, -1)
		var pager string
		if nrec > 0 {
			pager = pagination(c, query, nrec, idx, limit)
			page = fmt.Sprintf("%s<br><br>%s", page, pager)
		} else {
			page = fmt.Sprintf("%s<br><br>No results found</br>", page)
		}
		for _, rec := range records {
			oid := rec["_id"].(primitive.ObjectID)
			rec["_id"] = oid
			tmpl["Id"] = oid.Hex()
			tmpl["Did"] = rec["did"]
			tmpl["RecordString"] = rec.ToString()
			tmpl["Record"] = rec.ToJSON()
			tmpl["Description"] = fmt.Sprintf("update on %s", time.Now().String())
			prec := utils.TmplPage(StaticFs, "record.tmpl", tmpl)
			page = fmt.Sprintf("%s<br>%s", page, prec)
		}
		if nrec > 5 {
			page = fmt.Sprintf("%s<br><br>%s", page, pager)
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(server.Top(c) + page + server.Bottom(c)))
}

// DataHandler provides access to / and /data end-points
func DataHandler(c *gin.Context) {
	r := c.Request
	w := c.Writer
	user, _ := username(r)
	tmpl := server.MakeTmpl(c, "Data")
	tmpl["User"] = user
	tmpl["Date"] = time.Now().Unix()
	tmpl["Beamlines"] = _beamlines
	var forms []string
	for idx, fname := range srvConfig.Config.CHESSMetaData.SchemaFiles {
		cls := "hide"
		if idx == 0 {
			cls = ""
		}
		form, err := genForm(c, fname, nil)
		if err != nil {
			log.Println("ERROR", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		beamlineForm := fmt.Sprintf("<div id=\"%s\" class=\"%s\">%s</div>", utils.FileName(fname), cls, form)
		forms = append(forms, beamlineForm)
	}
	tmpl["Form"] = template.HTML(strings.Join(forms, "\n"))
	page := utils.TmplPage(StaticFs, "keys.tmpl", tmpl)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(server.Top(c) + page + server.Bottom(c)))
}
func UpdateRecordHandler(c *gin.Context) {
}
func UploadJsonHandler(c *gin.Context) {
}
func ProcessHandler(c *gin.Context) {
}
