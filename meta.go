package main

import (
	"log"

	srvConfig "github.com/CHESSComputing/common/config"
	oreMongo "github.com/CHESSComputing/common/mongo"
	bson "go.mongodb.org/mongo-driver/bson"
	// bson "gopkg.in/mgo.v2/bson"
)

// MetaData represents meta-data object
type MetaData struct {
	ID          string   `json:"id"`
	Site        string   `json:"site" binding:"required"`
	Description string   `json:"description" binding:"required"`
	Bucket      string   `json:"bucket" binding:"required"`
	Tags        []string `json:"tags"`
}

// Record converts MetaData to MongoDB record
func (m *MetaData) Record() oreMongo.Record {
	rec := make(oreMongo.Record)
	rec["id"] = m.ID
	rec["site"] = m.Site
	rec["description"] = m.Description
	rec["bucket"] = m.Bucket
	rec["tags"] = m.Tags
	return rec
}

// insert MetaData record to MongoDB
func (m *MetaData) mongoInsert() {
	var records []oreMongo.Record
	records = append(records, m.Record())
	oreMongo.Insert(
		srvConfig.Config.MetaData.MongoDB.DBName,
		srvConfig.Config.MetaData.MongoDB.DBColl,
		records)
}

// upsert MetaData record to MongoDB using given key
func (m *MetaData) mongoUpsert(key string) {
	var records []oreMongo.Record
	records = append(records, m.Record())
	oreMongo.Upsert(
		srvConfig.Config.MetaData.MongoDB.DBName,
		srvConfig.Config.MetaData.MongoDB.DBColl,
		key,
		records)
}

// remove MetaData record from MongoDB
func (m *MetaData) mongoRemove() {
	spec := bson.M{"id": m.ID}
	oreMongo.Remove(
		srvConfig.Config.MetaData.MongoDB.DBName,
		srvConfig.Config.MetaData.MongoDB.DBColl,
		spec)
}

// global list of existing meta-data records
// should be replaced with permistent MongoDB storage
var _metaData []MetaData

// helper function to return existing meta-data
func metadata(site string) []MetaData {
	// so far we will return our global _metaData list
	if srvConfig.Config.MetaData.WebServer.Verbose > 0 {
		log.Println("metadata for site=", site)
	}
	if site == "" {
		return _metaData
	}
	var out []MetaData
	for _, r := range _metaData {
		if srvConfig.Config.MetaData.WebServer.Verbose > 0 {
			log.Printf("MetaData record %+v matching site %s", r, site)
		}
		if r.Site == site {
			out = append(out, r)
		}
	}
	return out
}

// helper function to return existing meta-data
func getRecord(mid string) []MetaData {
	var out []MetaData
	// so far we will return our global _metaData list
	if srvConfig.Config.MetaData.WebServer.Verbose > 0 {
		log.Println("metadata for mid=", mid)
	}
	if mid == "" {
		return out
	}
	for _, r := range _metaData {
		if srvConfig.Config.MetaData.WebServer.Verbose > 0 {
			log.Printf("MetaData record %+v matching mid %s", r, mid)
		}
		if r.ID == mid {
			out = append(out, r)
		}
	}
	return out
}
