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
)

func main() {
	// Serve Block Requests
	go bcnetgo.Bind(bcgo.PORT_BLOCK, bcnetgo.HandleBlock)
	// Serve Head Requests
	go bcnetgo.Bind(bcgo.PORT_HEAD, bcnetgo.HandleHead)

	// Serve Web Requests
	http.HandleFunc("/", bcnetgo.HandleStatic)
	ks := &bcnetgo.KeyStore{
		Keys: make(map[string]*bcgo.KeyShare),
	}
	http.HandleFunc("/keys", ks.HandleKeys)
	http.HandleFunc("/status", bcnetgo.HandleStatus)
	// Serve HTTPS HTML Requests
	go log.Fatal(http.ListenAndServeTLS(":443", "server.crt", "server.key", nil))
	// Redirect HTTP HTML Requests to HTTPS
	log.Fatal(http.ListenAndServe(":80", http.HandlerFunc(bcnetgo.HTTPSRedirect)))
	/*
		// Serve HTTP HTML Requests
		log.Fatal(http.ListenAndServe(":80", nil))
	*/
}
