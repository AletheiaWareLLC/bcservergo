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
	"github.com/AletheiaWareLLC/aliasgo"
	"github.com/AletheiaWareLLC/aliasservergo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/bcnetgo"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"time"
)

func main() {
	rootDir, err := bcgo.GetRootDirectory()
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Root Dir:", rootDir)

	certDir, err := bcgo.GetCertificateDirectory(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Certificate Directory:", certDir)

	logFile, err := bcgo.SetupLogging(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	defer logFile.Close()
	log.Println("Log File:", logFile.Name())

	cacheDir, err := bcgo.GetCacheDirectory(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Cache Dir:", cacheDir)

	cache, err := bcgo.NewFileCache(cacheDir)
	if err != nil {
		log.Println(err)
		return
	}

	peers, err := bcgo.GetPeers(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Peers:", peers)

	network := &bcgo.TcpNetwork{
		Peers: peers,
	}

	node, err := bcgo.GetNode(rootDir, cache, network)
	if err != nil {
		log.Println(err)
		return
	}

	aliases := aliasgo.OpenAliasChannel()
	if err := bcgo.LoadHead(aliases, cache, network); err != nil {
		log.Println(err)
	}
	if err := bcgo.Pull(aliases, cache, network); err != nil {
		log.Println(err)
	}
	node.AddChannel(aliases)

	listener := &bcgo.PrintingMiningListener{os.Stdout}

	// Serve Block Requests
	go bcnetgo.Bind(bcgo.PORT_GET_BLOCK, bcnetgo.BlockPortHandler(cache, network))
	// Serve Head Requests
	go bcnetgo.Bind(bcgo.PORT_GET_HEAD, bcnetgo.HeadPortHandler(cache, network))
	// Serve Block Updates
	go bcnetgo.Bind(bcgo.PORT_BROADCAST, bcnetgo.BroadcastPortHandler(cache, network, func(name string) (bcgo.Channel, error) {
		return node.GetChannel(name)
	}))

	// Redirect HTTP Requests to HTTPS
	go http.ListenAndServe(":80", http.HandlerFunc(bcnetgo.HTTPSRedirect))

	// Serve Web Requests
	mux := http.NewServeMux()
	mux.HandleFunc("/", bcnetgo.StaticHandler)
	aliasTemplate, err := template.ParseFiles("html/template/alias.html")
	if err != nil {
		log.Println(err)
		return
	}
	mux.HandleFunc("/alias", aliasservergo.AliasHandler(aliases, cache, network, aliasTemplate))
	aliasRegistrationTemplate, err := template.ParseFiles("html/template/alias-register.html")
	if err != nil {
		log.Println(err)
		return
	}
	mux.HandleFunc("/alias-register", aliasservergo.AliasRegistrationHandler(aliases, node, listener, aliasRegistrationTemplate))
	blockTemplate, err := template.ParseFiles("html/template/block.html")
	if err != nil {
		log.Println(err)
		return
	}
	mux.HandleFunc("/block", bcnetgo.BlockHandler(cache, network, blockTemplate))
	channelTemplate, err := template.ParseFiles("html/template/channel.html")
	if err != nil {
		log.Println(err)
		return
	}
	mux.HandleFunc("/channel", bcnetgo.ChannelHandler(cache, network, channelTemplate))
	channelListTemplate, err := template.ParseFiles("html/template/channel-list.html")
	if err != nil {
		log.Println(err)
		return
	}
	mux.HandleFunc("/channels", bcnetgo.ChannelListHandler(cache, network, channelListTemplate, node.GetChannels))
	mux.HandleFunc("/keys", bcnetgo.KeyShareHandler(make(bcnetgo.KeyShareStore), 2*time.Minute))
	// Serve HTTPS Requests
	log.Println(http.ListenAndServeTLS(":443", path.Join(certDir, "fullchain.pem"), path.Join(certDir, "privkey.pem"), mux))
}
