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
	"encoding/base64"
	"github.com/AletheiaWareLLC/aliasgo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/bcnetgo"
	"github.com/golang/protobuf/proto"
	"html/template"
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
	go bcnetgo.Bind(bcgo.PORT_MULTICAST, bcnetgo.HandleUpdate)

	// Serve Web Requests
	mux := http.NewServeMux()
	mux.HandleFunc("/", bcnetgo.HandleStatic)
	mux.HandleFunc("/alias", HandleAlias)
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

func HandleAlias(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
	aliases, err := aliasgo.OpenAliasChannel()
	if err != nil {
		log.Println(err)
		return
	}
	switch r.Method {
	case "GET":
		query := r.URL.Query()
		var a string
		if results, ok := query["alias"]; ok && len(results) == 1 {
			a = results[0]
		}
		log.Println("Alias", a)
		if err := aliasgo.UniqueAlias(aliases, a); err != nil {
			log.Println(err)
			// TODO warn user
			return
		}
		var publicKey string
		if results, ok := query["publicKey"]; ok && len(results) == 1 {
			publicKey = results[0]
		}
		log.Println("PublicKey", publicKey)
		t, err := template.ParseFiles("html/template/alias.html")
		if err != nil {
			log.Println(err)
			return
		}
		data := struct {
			Alias     string
			PublicKey string
		}{
			Alias:     a,
			PublicKey: publicKey,
		}
		log.Println("Data", data)
		err = t.Execute(w, data)
		if err != nil {
			log.Println(err)
			return
		}
	case "POST":
		r.ParseForm()
		log.Println("Request", r)
		a := r.Form["alias"]
		log.Println("Alias", a)
		publicKey := r.Form["publicKey"]
		log.Println("PublicKey", publicKey)
		publicKeyFormat := r.Form["publicKeyFormat"]
		log.Println("PublicKeyFormat", publicKeyFormat)
		signature := r.Form["signature"]
		log.Println("Signature", signature)
		signatureAlgorithm := r.Form["signatureAlgorithm"]
		log.Println("SignatureAlgorithm", signatureAlgorithm)

		if err := aliasgo.UniqueAlias(aliases, a[0]); err != nil {
			log.Println(err)
			return
		}

		pubKey, err := base64.RawURLEncoding.DecodeString(publicKey[0])
		if err != nil {
			log.Println(err)
			return
		}

		pubFormatValue, ok := bcgo.PublicKeyFormat_value[publicKeyFormat[0]]
		if !ok {
			log.Println("Unrecognized Public Key Format")
			return
		}
		pubFormat := bcgo.PublicKeyFormat(pubFormatValue)

		sig, err := base64.RawURLEncoding.DecodeString(signature[0])
		if err != nil {
			log.Println(err)
			return
		}

		sigAlgValue, ok := bcgo.SignatureAlgorithm_value[signatureAlgorithm[0]]
		if !ok {
			log.Println("Unrecognized Signature")
			return
		}
		sigAlg := bcgo.SignatureAlgorithm(sigAlgValue)

		record, err := aliasgo.CreateAliasRecord(a[0], pubKey, pubFormat, sig, sigAlg)
		if err != nil {
			log.Println(err)
			return
		}

		data, err := proto.Marshal(record)
		if err != nil {
			log.Println(err)
			return
		}

		entries := [1]*bcgo.BlockEntry{
			&bcgo.BlockEntry{
				RecordHash: bcgo.Hash(data),
				Record:     record,
			},
		}

		node, err := bcgo.GetNode()
		if err != nil {
			log.Println(err)
			return
		}

		// Mine record into blockchain
		hash, block, err := node.MineRecords(aliases, entries[:])
		if err != nil {
			log.Println(err)
			return
		}
		if err := aliases.Multicast(hash, block); err != nil {
			log.Println(err)
			return
		}
	default:
		log.Println("Unsupported method", r.Method)
	}
}
