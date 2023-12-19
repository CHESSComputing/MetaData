package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	beamlines "github.com/CHESSComputing/golib/beamlines"
	srvConfig "github.com/CHESSComputing/golib/config"
	mongo "github.com/CHESSComputing/golib/mongo"
	server "github.com/CHESSComputing/golib/server"
	services "github.com/CHESSComputing/golib/services"
	utils "github.com/CHESSComputing/golib/utils"
	"github.com/gin-gonic/gin"
	bson "go.mongodb.org/mongo-driver/bson"
)

// MetaParams represents URI storage params in /did end-point
type MetaParams struct {
	DID int64 `uri:"did" binding:"required"`
}

// helper function to provide error page
func handleError(c *gin.Context, msg string, err error) {
	page := server.ErrorPage(StaticFs, msg, err)
	w := c.Writer
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(header() + page + footer()))
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
	c.JSON(http.StatusOK, records)
}

// RecordHandler handles POST queries
func RecordHandler(c *gin.Context) {
	var params MetaParams
	err := c.ShouldBindUri(&params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "fail", "error": err.Error()})
		return
	}
	var records []mongo.Record
	spec := bson.M{"did": params.DID}
	records = mongo.Get(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec, 0, -1)
	if Verbose > 0 {
		log.Println("RecordHandler", spec, records)
	}
	c.JSON(http.StatusOK, records)
}

// helper function to parse input HTTP request JSON spec data
func parseSpec(c *gin.Context) (mongo.Record, error) {
	var rec mongo.Record
	defer c.Request.Body.Close()
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return rec, err
	}
	err = json.Unmarshal(body, &rec)
	if err != nil {
		return rec, err
	}
	if Verbose > 0 {
		log.Printf("QueryHandler received request %+v", rec)
	}
	return rec, nil
}

// helper function to parse input HTTP request JSON data
func parseRequest(c *gin.Context) (services.MetaRecord, error) {
	var rec services.MetaRecord
	defer c.Request.Body.Close()
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return rec, err
	}
	err = json.Unmarshal(body, &rec)
	if err != nil {
		return rec, err
	}
	if Verbose > 0 {
		log.Printf("QueryHandler received request %+v", rec)
	}
	return rec, nil
}

// DataHandler handles POST upload of meta-data record
func DataHandler(c *gin.Context) {
	rec, err := parseRequest(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "status": "fail"})
		return
	}
	sname := rec.Schema
	if sname == "" {
		msg := "No schema found in meta-data record"
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.New(msg), "status": "fail"})
		return
	}
	schema := beamlines.SchemaFileName(sname)
	record := rec.Record
	if Verbose > 0 {
		log.Printf("insert schema=%s record=%+v", schema, record)
	}
	// insert record to meta-data database
	err = insertData(schema, record)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "status": "fail"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// QueryHandler handles POST queries
func QueryHandler(c *gin.Context) {
	rec, err := parseSpec(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "status": "fail"})
		return
	}
	query := fmt.Sprintf("%v", rec["query"])
	user := fmt.Sprintf("%v", rec["user"])
	idx := 0
	limit := 10
	if val, ok := rec["idx"]; ok {
		if v, err := strconv.Atoi(fmt.Sprintf("%v", val)); err == nil {
			idx = v
		}
	}
	if val, ok := rec["limit"]; ok {
		if v, err := strconv.Atoi(fmt.Sprintf("%v", val)); err == nil {
			limit = v
		}
	}
	spec, err := ParseQuery(query)
	if Verbose > 0 {
		log.Printf("search query='%s' spec=%+v user=%v", query, spec, user)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "status": "fail"})
		return
	}

	var records []mongo.Record
	nrecords := 0
	if spec != nil {
		nrecords = mongo.Count(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec)
		records = mongo.Get(srvConfig.Config.CHESSMetaData.DBName, srvConfig.Config.CHESSMetaData.DBColl, spec, idx, limit)
	}
	response := services.MetaResponse{
		Query: query, Spec: spec, Idx: idx, Limit: limit, NRecords: nrecords, Records: records,
	}
	if Verbose > 0 {
		log.Printf("spec %v nrecords %d return idx=%d limit=%d", spec, nrecords, idx, limit)
	}
	c.JSON(http.StatusOK, response)
}

// DeleteHandler handles POST queries
func DeleteHandler(c *gin.Context) {
}

// UploadJsonHandler handles upload of JSON record
func UploadJsonHandler(c *gin.Context) {
	r := c.Request
	w := c.Writer

	// get beamline value from the form
	sname := r.FormValue("SchemaName")

	// read form file
	file, _, err := r.FormFile("file")
	if err != nil {
		msg := "unable to read file form"
		handleError(c, msg, err)
		return
	}
	defer file.Close()

	defer r.Body.Close()
	body, err := ioutil.ReadAll(file)
	var rec mongo.Record
	if err == nil {
		err = json.Unmarshal(body, &rec)
		if err != nil {
			log.Println("unable to read HTTP JSON record, error:", err)
		}
	}
	user, _ := username(r)
	tmpl := server.MakeTmpl(StaticFs, "Upload")
	tmpl["User"] = user
	tmpl["Date"] = time.Now().Unix()
	schemaFiles := srvConfig.Config.CHESSMetaData.SchemaFiles
	if sname != "" {
		// construct proper schema files order which will be used to generate forms
		sfiles := []string{}
		// add scheme file which matches our desired schema
		for _, f := range schemaFiles {
			if strings.Contains(f, sname) {
				sfiles = append(sfiles, f)
			}
		}
		// add rest of schema files
		for _, f := range schemaFiles {
			if !strings.Contains(f, sname) {
				sfiles = append(sfiles, f)
			}
		}
		schemaFiles = sfiles
		// construct proper bemalines order
		blines := []string{sname}
		for _, b := range _beamlines {
			if b != sname {
				blines = append(blines, b)
			}
		}
		tmpl["Beamlines"] = blines
	} else {
		tmpl["Beamlines"] = _beamlines
	}
	var forms []string
	for idx, fname := range schemaFiles {
		cls := "hide"
		if idx == 0 {
			cls = ""
		}
		form, err := genForm(c, fname, &rec)
		if err != nil {
			log.Println("ERROR", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		beamlineForm := fmt.Sprintf("<div id=\"%s\" class=\"%s\">%s</div>", utils.FileName(fname), cls, form)
		forms = append(forms, beamlineForm)
	}
	tmpl["Form"] = template.HTML(strings.Join(forms, "\n"))
	page := server.TmplPage(StaticFs, "keys.tmpl", tmpl)
	w.Write([]byte(header() + page + footer()))
}

// ProcessHandler handlers Process requests
func ProcessHandler(c *gin.Context) {
	r := c.Request
	w := c.Writer
	var msg string
	var class string
	tmpl := server.MakeTmpl(StaticFs, "Process")
	user, _ := username(r)
	tmpl["User"] = user
	if err := r.ParseForm(); err == nil {
		schema, rec, err := processForm(r)
		// save parsed record for later usage
		if data, e := json.MarshalIndent(rec, "", "   "); e == nil {
			tmpl["JsonRecord"] = template.HTML(string(data))
		}
		if err != nil {
			msg = fmt.Sprintf("Web processing error: %v", err)
			class = "alert alert-error"
			tmpl["Message"] = msg
			tmpl["Class"] = class
			page := server.TmplPage(StaticFs, "confirm.tmpl", tmpl)
			w.Write([]byte(header() + page + footer()))
			return
		}
		err = insertData(schema, rec)
		if err == nil {
			msg = fmt.Sprintf("Your meta-data is inserted successfully")
			log.Println("INFO", msg)
			class = "alert alert-success"
		} else {
			//             msg = fmt.Sprintf("Web processing error: %v", err)
			msg = fmt.Sprintf("ERROR: %v", err)
			class = "alert alert-error"
			log.Println("WARNING", msg)
			tmpl["Schema"] = schemaName(schema)
			tmpl["Message"] = msg
			tmpl["Class"] = class
			page := server.TmplPage(StaticFs, "confirm.tmpl", tmpl)
			// redirect users to update their record
			inputs := htmlInputs(rec)
			tmpl["Inputs"] = inputs
			tmpl["Id"] = ""
			tmpl["Description"] = fmt.Sprintf("update on %s", time.Now().String())
			page += server.TmplPage(StaticFs, "update.tmpl", tmpl)
			w.Write([]byte(header() + page + footer()))
			return
		}
	}
	tmpl["Message"] = msg
	tmpl["Class"] = class
	page := server.TmplPage(StaticFs, "confirm.tmpl", tmpl)
	w.Write([]byte(header() + page + footer()))
}

// APIHandler handlers Api requests
func APIHandler(c *gin.Context) {
	w := c.Writer
	r := c.Request

	// read schema name from web form
	var schema string
	sname := r.FormValue("SchemaName")
	if sname != "" {
		schema = schemaFileName(sname)
	} else { // we got CLI request
		if items, ok := r.URL.Query()["schema"]; ok {
			sname = items[0]
		}
	}
	if sname == "" {
		msg := "client does not provide schema name"
		handleError(c, msg, errors.New("Bad request"))
		return
	}
	if Verbose > 0 {
		log.Printf("APIHandler schema=%s, file=%s", sname, schema)
	}

	user, err := userCredentials(r)
	if err != nil {
		msg := "unable to get user credentials"
		handleError(c, msg, err)
		return
	}
	if !srvConfig.Config.CHESSMetaData.TestMode {

		// check if we got request from CLI, we proceed here only for web clients
		userAgent := r.Header.Get("User-Agent")
		if !strings.Contains(userAgent, "Go-http-client") {
			config := r.FormValue("config")
			var data = mongo.Record{}
			data["User"] = user
			err = json.Unmarshal([]byte(config), &data)
			if err != nil {
				msg := "unable to unmarshal configuration data"
				handleError(c, msg, err)
				return
			}
			err = insertData(schema, data)
			if err != nil {
				msg := "unable to insert data"
				handleError(c, msg, err)
				return
			}
			msg := fmt.Sprintf("Successfully inserted:\n%v", data.ToString())
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(msg))
			return
		}
	}

	// our data record
	var data = mongo.Record{}
	data["User"] = user

	// process cli request
	record := r.FormValue("record")
	if record != "" {
		err := json.Unmarshal([]byte(record), &data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "status": "fail"})
			return
		}
		err = insertData(schema, data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "status": "fail"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	// process web form
	file, _, err := r.FormFile("file")
	if err != nil {
		msg := "unable to read file form"
		handleError(c, msg, err)
		return
	}
	defer file.Close()

	var msg, class string
	defer r.Body.Close()
	body, err := io.ReadAll(file)
	if err != nil {
		msg = fmt.Sprintf("error: %v, unable to read request data", err)
		class = "alert alert-error"
	} else {
		log.Println("body", string(body))
		err := json.Unmarshal(body, &data)
		if err != nil {
			msg = fmt.Sprintf("error: %v, unable to parse request data", err)
			class = "alert alert-error"
		} else {
			err := insertData(schema, data)
			if err == nil {
				msg = fmt.Sprintf("meta-data is inserted successfully")
				class = "alert alert-success"
			} else {
				msg = fmt.Sprintf("ERROR: %v", err)
				class = "alert alert-error"
			}
		}
	}
	tmpl := server.MakeTmpl(StaticFs, "Api")
	tmpl["Schema"] = schemaName(schema)
	tmpl["Message"] = msg
	tmpl["Class"] = class
	page := server.TmplPage(StaticFs, "confirm.tmpl", tmpl)
	w.Write([]byte(header() + page + footer()))
}

// UpdateHandler handlers Process requests
func UpdateHandler(c *gin.Context) {
	r := c.Request
	w := c.Writer
	tmpl := server.MakeTmpl(StaticFs, "Update")
	record := r.FormValue("record")
	var rec mongo.Record
	err := json.Unmarshal([]byte(record), &rec)
	if err != nil {
		msg := "unable to unmarshal passed record"
		handleError(c, msg, err)
		return
	}
	// we will prepare input entries for the template
	// where each entry represented in form of template.HTML
	// to avoid escaping of HTML characters
	inputs := htmlInputs(rec)
	tmpl["Inputs"] = inputs
	tmpl["Id"] = r.FormValue("_id")
	tmpl["Description"] = fmt.Sprintf("update on %s", time.Now().String())
	page := server.TmplPage(StaticFs, "update.tmpl", tmpl)
	w.Write([]byte(header() + page + footer()))
}

// UpdateRecordHandler handlers Process requests
func UpdateRecordHandler(c *gin.Context) {
	r := c.Request
	w := c.Writer
	tmpl := server.MakeTmpl(StaticFs, "UpdateRecord")
	user, _ := username(r)
	tmpl["User"] = user
	var msg, cls, schema string
	var rec mongo.Record
	if err := r.ParseForm(); err == nil {
		schema, rec, err = processForm(r)
		if err != nil {
			msg := fmt.Sprintf("Web processing error: %v", err)
			class := "alert alert-error"
			tmpl["Message"] = msg
			tmpl["Class"] = class
			page := server.TmplPage(StaticFs, "confirm.tmpl", tmpl)
			w.Write([]byte(header() + page + footer()))
			return
		}
		rid := r.FormValue("_id")
		// delete record id before the update
		delete(rec, "_id")
		if rid == "" {
			err := insertData(schema, rec)
			if err == nil {
				msg = fmt.Sprintf("Your meta-data is inserted successfully")
				cls = "alert alert-success"
			} else {
				//                 msg = fmt.Sprintf("update web processing error: %v", err)
				msg = fmt.Sprintf("ERROR: %v", err)
				cls = "alert alert-error"
			}
		} else {
			msg = fmt.Sprintf("record %v is successfully updated", rid)
			log.Println("MongoUpsert", rec)
			records := []mongo.Record{rec}
			err = mongo.Upsert(
				srvConfig.Config.CHESSMetaData.MongoDB.DBName,
				srvConfig.Config.CHESSMetaData.MongoDB.DBColl,
				"dataset", records)
			if err != nil {
				msg = fmt.Sprintf("record %v update is failed, reason: %v", rid, err)
				cls = "alert-error"
			} else {
				cls = "alert-success"
			}
		}
	} else {
		msg = fmt.Sprintf("record update failed, reason: %v", err)
		cls = "alert-error"
	}
	tmpl["Schema"] = schemaName(schema)
	tmpl["Message"] = strings.ToTitle(msg)
	tmpl["Class"] = fmt.Sprintf("alert %s text-large centered", cls)
	page := server.TmplPage(StaticFs, "confirm.tmpl", tmpl)
	w.Write([]byte(header() + page + footer()))
}

// FilesHandler handlers Files requests
func FilesHandler(c *gin.Context) {
	r := c.Request
	w := c.Writer
	_, err := userCredentials(r)
	if err != nil {
		msg := "unable to get user credentials"
		handleError(c, msg, err)
		return
	}
	if !srvConfig.Config.CHESSMetaData.TestMode && err != nil {
		did, err := strconv.ParseInt(r.FormValue("did"), 10, 64)
		if err != nil {
			msg := fmt.Sprintf("Unable to parse did\nError: %v", err)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(msg))
			return
		}
		files, err := getFiles(did)
		if err != nil {
			msg := fmt.Sprintf("Unable to get files\nError: %v", err)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(msg))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(strings.Join(files, "\n")))
		return
	}
	tmpl := server.MakeTmpl(StaticFs, "Files")
	did, err := strconv.ParseInt(r.FormValue("did"), 10, 64)
	if err != nil {
		tmpl["Message"] = fmt.Sprintf("Unable to parse did\nError: %v", err)
		tmpl["Class"] = "alert alert-error text-large centered"
		page := server.TmplPage(StaticFs, "confirm.tmpl", tmpl)
		w.Write([]byte(header() + page + footer()))
		return
	}
	files, err := getFiles(did)
	if err != nil {
		tmpl["Message"] = fmt.Sprintf("Unable to query FilesDB\nError: %v", err)
		tmpl["Class"] = "alert alert-error text-large centered"
		page := server.TmplPage(StaticFs, "confirm.tmpl", tmpl)
		w.Write([]byte(header() + page + footer()))
		return
	}
	tmpl["Id"] = r.FormValue("_id")
	tmpl["Did"] = did
	tmpl["Files"] = strings.Join(files, "\n")
	page := server.TmplPage(StaticFs, "files.tmpl", tmpl)
	w.Write([]byte(header() + page + footer()))
}
