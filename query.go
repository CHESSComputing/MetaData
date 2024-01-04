package main

// query module
//
// Copyright (c) 2019 - Valentin Kuznetsov <vkuznet@gmail.com>
//

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	utils "github.com/CHESSComputing/golib/utils"
	bson "go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// separator defines our query separator
var separator = ":"

// ParseQuery function provides basic parser for user queries and return
// results in bson dictionary
func ParseQuery(query string) (bson.M, error) {
	spec := make(bson.M)
	if strings.TrimSpace(query) == "" {
		log.Println("WARNING: empty query string")
		return nil, errors.New("empty query")
	}
	// support MongoDB specs
	if strings.Contains(query, "{") {
		err := json.Unmarshal([]byte(query), &spec)
		if err == nil {
			if Verbose > 0 {
				log.Printf("found bson spec %+v", spec)
			}
			// adjust query _id to object id type
			if val, ok := spec["_id"]; ok {
				if oid, err := primitive.ObjectIDFromHex(val.(string)); err == nil {
					spec["_id"] = oid
				}
			}
			return spec, nil
		}
		log.Printf("ERROR: unable to parse input query '%s' error %v", query, err)
		return nil, err
	}

	// query as key:value
	if strings.Contains(query, separator) {
		arr := strings.Split(query, separator)
		var vals []string
		key := arr[0]
		last := arr[len(arr)-1]
		for i := 0; i < len(arr); i++ {
			if len(arr) > i+1 {
				vals = strings.Split(arr[i+1], " ")
				if arr[i+1] == last {
					spec[key] = last
					break
				}
				if len(vals) > 0 {
					values := strings.Join(vals[:len(vals)-1], " ")
					spec[key] = values
					key = vals[len(vals)-1]
				} else {
					spec[key] = vals[0]
					break
				}
			} else {
				vals = arr[i:]
				values := strings.Join(vals, " ")
				spec[key] = values
				break
			}
		}
	} else {
		// or, query as free text
		spec["$text"] = bson.M{"$search": query}
	}
	return adjustQuery(spec), nil
}

// helper function to adjust query keys
func adjustQuery(spec bson.M) bson.M {
	// TODO: take input query and change its keys to match schema
	nspec := make(bson.M)
	for kkk, val := range spec {
		if strings.HasPrefix(kkk, "$") {
			continue
		}
		// adjust query _id to object id type
		if kkk == "_id" {
			if oid, err := primitive.ObjectIDFromHex(val.(string)); err == nil {
				nspec["_id"] = oid
			}
			continue
		}
		// look-up appropriate schema key
		if key, ok := _schemaKeys[strings.ToLower(kkk)]; ok {
			// create regex for value if it is the string
			sval := fmt.Sprintf("%v", val)
			if utils.PatternInt.MatchString(sval) || utils.PatternFloat.MatchString(sval) {
				nspec[key] = val
			} else {
				//                 pat, err := regexp.Compile(fmt.Sprintf("/^%s$/i", sval))
				pat := fmt.Sprintf("^%s$", sval)
				nspec[key] = bson.M{"$regex": pat, "$options": "i"}
			}
		} else {
			if kkk != "did" {
				log.Printf("WARNING: unable to find matching schema key for %s, existing schema keys %+v", kkk, _schemaKeys)
			}
			nspec[kkk] = val
		}
	}
	if Verbose > 0 {
		log.Printf("Perform adjustment of input query from %+v to %+v", spec, nspec)
	}
	return nspec
}
