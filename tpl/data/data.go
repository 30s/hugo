// Copyright 2017 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package data

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gohugoio/hugo/deps"
)

// New returns a new instance of the data-namespaced template functions.
func New(deps *deps.Deps) *Namespace {
	return &Namespace{
		deps:   deps,
		client: http.DefaultClient,
	}
}

// Namespace provides template functions for the "data" namespace.
type Namespace struct {
	deps *deps.Deps

	client *http.Client
}

// GetCSV expects a data separator and one or n-parts of a URL to a resource which
// can either be a local or a remote one.
// The data separator can be a comma, semi-colon, pipe, etc, but only one character.
// If you provide multiple parts for the URL they will be joined together to the final URL.
// GetCSV returns nil or a slice slice to use in a short code.
func (ns *Namespace) GetCSV(sep string, urlParts ...string) (d [][]string, err error) {
	url := strings.Join(urlParts, "")

	var clearCacheSleep = func(i int, u string) {
		ns.deps.Log.WARN.Printf("Retry #%d for %s and sleeping for %s", i, url, resSleep)
		time.Sleep(resSleep)
		deleteCache(url, ns.deps.Fs.Source, ns.deps.Cfg)
	}

	for i := 0; i <= resRetries; i++ {
		var req *http.Request
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("Failed to create request for getCSV for resource %s: %s", url, err)
		}

		req.Header.Add("Accept", "text/csv")
		req.Header.Add("Accept", "text/plain")

		var c []byte
		c, err = ns.getResource(req)
		if err != nil {
			ns.deps.Log.ERROR.Printf("Failed to read CSV resource %q: %s", url, err)
			return nil, nil
		}

		if !bytes.Contains(c, []byte(sep)) {
			ns.deps.Log.ERROR.Printf("Cannot find separator %s in CSV for %s", sep, url)
			return nil, nil
		}

		if d, err = parseCSV(c, sep); err != nil {
			ns.deps.Log.WARN.Printf("Failed to parse CSV file %s: %s", url, err)
			clearCacheSleep(i, url)
			continue
		}
		break
	}

	if err != nil {
		ns.deps.Log.ERROR.Printf("Failed to read CSV resource %q: %s", url, err)
		return nil, nil
	}

	return
}

// GetJSON expects one or n-parts of a URL to a resource which can either be a local or a remote one.
// If you provide multiple parts they will be joined together to the final URL.
// GetJSON returns nil or parsed JSON to use in a short code.
func (ns *Namespace) GetJSON(urlParts ...string) (v interface{}, err error) {
	url := strings.Join(urlParts, "")

	for i := 0; i <= resRetries; i++ {
		var req *http.Request
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("Failed to create request for getJSON resource %s: %s", url, err)
		}

		req.Header.Add("Accept", "application/json")

		var c []byte
		c, err = ns.getResource(req)
		if err != nil {
			ns.deps.Log.ERROR.Printf("Failed to get JSON resource %s: %s", url, err)
			return nil, nil
		}

		err = json.Unmarshal(c, &v)
		if err != nil {
			ns.deps.Log.WARN.Printf("Cannot read JSON from resource %s: %s", url, err)
			ns.deps.Log.WARN.Printf("Retry #%d for %s and sleeping for %s", i, url, resSleep)
			time.Sleep(resSleep)
			deleteCache(url, ns.deps.Fs.Source, ns.deps.Cfg)
			continue
		}
		break
	}

	if err != nil {
		ns.deps.Log.ERROR.Printf("Failed to get JSON resource %s: %s", url, err)
		return nil, nil
	}
	return
}

// parseCSV parses bytes of CSV data into a slice slice string or an error
func parseCSV(c []byte, sep string) ([][]string, error) {
	if len(sep) != 1 {
		return nil, errors.New("Incorrect length of csv separator: " + sep)
	}
	b := bytes.NewReader(c)
	r := csv.NewReader(b)
	rSep := []rune(sep)
	r.Comma = rSep[0]
	r.FieldsPerRecord = 0
	return r.ReadAll()
}
