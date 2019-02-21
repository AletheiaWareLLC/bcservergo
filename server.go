/*
 * Copyright 2019 Aletheia Ware LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/bcnetgo"
	"log"
	"net/http"
	"path"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// Serve Block Requests
	go bcnetgo.Bind(bcgo.PORT_BLOCK, bcnetgo.HandleBlock)
	// Serve Head Requests
	go bcnetgo.Bind(bcgo.PORT_HEAD, bcnetgo.HandleHead)
	// Serve Block Updates
	go bcnetgo.Bind(bcgo.PORT_CAST, bcnetgo.HandleCast)

	// Serve Web Requests
	mux := http.NewServeMux()
	mux.HandleFunc("/", bcnetgo.HandleStatic)
	mux.HandleFunc("/alias", bcnetgo.HandleAlias)
	ks := &bcnetgo.KeyStore{
		Keys: make(map[string]*bcgo.KeyShare),
	}
	mux.HandleFunc("/keys", ks.HandleKeys)
	store, err := bcnetgo.GetSecurityStore()
	if err != nil {
		log.Println(err)
		return
	}
	// Serve HTTPS Requests
	log.Fatal(http.ListenAndServeTLS(":443", path.Join(store, "fullchain.pem"), path.Join(store, "privkey.pem"), mux))

	// TODO Redirect HTTP Requests to HTTPS
	// log.Fatal(http.ListenAndServe(":80", http.HandlerFunc(bcnetgo.HTTPSRedirect)))
}
