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
	logFile, err := bcnetgo.SetupLogging()
	if err != nil {
		log.Println(err)
		return
	}
	defer logFile.Close()

	// Serve Block Requests
	go bcnetgo.Bind(bcgo.PORT_BLOCK, bcnetgo.HandleBlockPort)
	// Serve Head Requests
	go bcnetgo.Bind(bcgo.PORT_HEAD, bcnetgo.HandleHeadPort)
	// Serve Block Updates
	go bcnetgo.Bind(bcgo.PORT_CAST, bcnetgo.HandleCastPort)

	// Redirect HTTP Requests to HTTPS
	go http.ListenAndServe(":80", http.HandlerFunc(bcnetgo.HTTPSRedirect))

	// Serve Web Requests
	mux := http.NewServeMux()
	mux.HandleFunc("/", bcnetgo.HandleStatic)
	mux.HandleFunc("/alias", bcnetgo.HandleAlias)
	mux.HandleFunc("/alias-register", bcnetgo.HandleAliasRegister)
	mux.HandleFunc("/block", bcnetgo.HandleBlock)
	mux.HandleFunc("/channel", bcnetgo.HandleChannel)
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
	log.Println(http.ListenAndServeTLS(":443", path.Join(store, "fullchain.pem"), path.Join(store, "privkey.pem"), mux))
}
